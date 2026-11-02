package wire

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	assistantapp "sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
	automationsapp "sweveh/internal/automations/app"
	"sweveh/internal/customers"
	customershttp "sweveh/internal/customers/adapters/in/http"
	messagingapp "sweveh/internal/messaging/app"
	msgdomain "sweveh/internal/messaging/domain"
	ordersapp "sweveh/internal/orders/app"
	ordersdomain "sweveh/internal/orders/domain"
	"sweveh/internal/shared"
	vehiclesapp "sweveh/internal/vehicles/app"
)

// taller implementa assistantapp.Taller sobre las fachadas y servicios ya cableados.
type taller struct{ s *Services }

var agent = shared.Actor{Type: shared.ActorAgent}

func (t *taller) Message(ctx context.Context, tallerID, id shared.ID) (assistantapp.Inbound, error) {
	m, err := t.s.Messaging.GetMessage(ctx, tallerID, id)
	if err != nil {
		return assistantapp.Inbound{}, err
	}
	c, err := t.s.Messaging.ConversationByID(ctx, tallerID, m.ConversationID)
	if err != nil {
		return assistantapp.Inbound{}, err
	}
	var p struct {
		Boton  string `json:"boton"`
		Nombre string `json:"nombre"`
	}
	_ = json.Unmarshal(m.Payload, &p)
	return assistantapp.Inbound{ID: m.ID, ConversationID: c.ID, Phone: c.Phone, Kind: m.Kind, Text: m.Body, ButtonID: p.Boton,
		ProfileName: p.Nombre, Mode: c.Mode, At: m.CreatedAt}, nil
}

func (t *taller) PreviousText(ctx context.Context, tallerID, convID shared.ID, before time.Time) (*assistantapp.Inbound, error) {
	_, msgs, err := t.s.Messaging.Messages(ctx, tallerID, convID)
	if err != nil {
		return nil, err
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Direction == "entrante" && m.Kind == "texto" && m.CreatedAt.Before(before) {
			in, err := t.Message(ctx, tallerID, m.ID)
			return &in, err
		}
	}
	return nil, nil
}

func (t *taller) ConsentAsked(ctx context.Context, tallerID, convID shared.ID) (bool, error) {
	_, msgs, err := t.s.Messaging.Messages(ctx, tallerID, convID)
	for _, m := range msgs {
		if m.Automation != nil && *m.Automation == "consentimiento" && m.Direction == "saliente" {
			return true, err
		}
	}
	return false, err
}

func (t *taller) Send(ctx context.Context, tallerID shared.ID, phone, text string, buttons []assistantapp.Button, automation string, orderID *shared.ID) (bool, error) {
	btns := make([]msgdomain.Button, 0, len(buttons))
	for _, b := range buttons {
		btns = append(btns, msgdomain.Button{ID: b.ID, Title: b.Title})
	}
	auto := automation
	_, err := t.s.Messaging.Enqueue(ctx, messagingapp.Outgoing{TallerID: tallerID, Phone: phone, Text: text, Buttons: btns,
		Origin: "agente", OrderID: orderID, Automation: &auto})
	if errors.Is(err, messagingapp.ErrOutsideWindow) {
		return false, nil
	}
	return err == nil, err
}

func (t *taller) SetMode(ctx context.Context, tallerID, convID shared.ID, mode string, wants bool) error {
	return t.s.Messaging.SetMode(ctx, tallerID, convID, mode, wants)
}

func (t *taller) LinkCustomer(ctx context.Context, tallerID, convID, customerID shared.ID) error {
	return t.s.Messaging.LinkCustomer(ctx, tallerID, convID, customerID)
}

func cust(c customers.Summary) assistantapp.Customer {
	return assistantapp.Customer{ID: c.ID, Name: c.Name, Phone: c.Phone, Consent: c.Consent}
}

func (t *taller) CustomerByPhone(ctx context.Context, tallerID shared.ID, phone string) (*assistantapp.Customer, error) {
	c, err := t.s.M.CustDir.ByPhone(ctx, tallerID, phone)
	if err != nil || c == nil {
		return nil, err
	}
	x := cust(*c)
	return &x, nil
}

func (t *taller) RegisterCustomer(ctx context.Context, tallerID shared.ID, phone, name string) (assistantapp.Customer, error) {
	c, err := t.s.M.CustDir.Register(ctx, tallerID, phone, name, agent)
	return cust(c), err
}

func (t *taller) SetConsent(ctx context.Context, tallerID, customerID shared.ID, granted bool, ref string, at time.Time) error {
	return t.s.M.CustDir.SetConsent(ctx, tallerID, customerID, granted, customers.ConsentRecord{
		Text: customershttp.ConsentText, Version: customershttp.ConsentVersion, Channel: "whatsapp", Ref: ref, At: at}, agent)
}

func (t *taller) OpenOrders(ctx context.Context, tallerID, customerID shared.ID) ([]assistantapp.Order, error) {
	var out []assistantapp.Order
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		vs, err := t.s.M.VehDir.ByCustomer(ctx, tallerID, customerID)
		if err != nil {
			return err
		}
		ids := make([]shared.ID, 0, len(vs))
		for _, v := range vs {
			ids = append(ids, v.ID)
		}
		list, err := t.s.M.OrdersF.OpenByCustomer(ctx, tallerID, customerID, ids)
		for _, i := range list {
			out = append(out, assistantapp.Order{ID: i.ID, Code: i.Code, Plate: i.Plate, Vehicle: i.Vehicle, State: i.State, PromisedAt: i.PromisedAt})
		}
		return err
	})
	return out, err
}

func (t *taller) Order(ctx context.Context, tallerID, orderID shared.ID) (assistantapp.Order, error) {
	var out assistantapp.Order
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		i, err := t.s.M.OrdersF.Info(ctx, tallerID, orderID)
		out = assistantapp.Order{ID: i.ID, Code: i.Code, Plate: i.Plate, Vehicle: i.Vehicle, State: i.State, PromisedAt: i.PromisedAt}
		return err
	})
	return out, err
}

func (t *taller) StatusReply(ctx context.Context, tallerID shared.ID, o assistantapp.Order, c assistantapp.Customer) (string, bool, error) {
	return t.s.Automations.StatusReply(ctx, tallerID, automationsapp.OrderInfo{ID: o.ID, Code: o.Code, State: o.State, Plate: o.Plate,
		Vehicle: o.Vehicle, PromisedAt: o.PromisedAt}, automationsapp.Customer{Name: c.Name, Phone: c.Phone, Consent: c.Consent})
}

func (t *taller) AgentOn(ctx context.Context, tallerID shared.ID) (bool, error) {
	cards, err := t.s.Automations.Cards(ctx, tallerID)
	for _, c := range cards {
		if c.Code == "AU-04" {
			return c.Active, err
		}
	}
	return false, err
}

func (t *taller) RespondQuote(ctx context.Context, tallerID, orderID shared.ID, decision, ref string) error {
	return t.s.M.QuotesF.Respond(ctx, tallerID, orderID, decision, "whatsapp", ref, agent)
}

// CreateOrderFromPlate crea vehículo (motor pendiente) y orden "Por recibir" (HU-55, HU-91).
func (t *taller) CreateOrderFromPlate(ctx context.Context, tallerID, customerID shared.ID, plate, symptom string) (string, error) {
	var code string
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		v, err := t.s.M.Vehicles.Register(ctx, vehiclesapp.RegisterVehicle{TallerID: tallerID, Actor: agent, Plate: plate, CustomerID: customerID})
		if err != nil {
			return err
		}
		o, err := t.s.M.Orders.Open(ctx, ordersapp.OpenOrder{TallerID: tallerID, Actor: agent, VehicleID: v.Vehicle.ID, Symptom: symptom,
			InitialState: ordersdomain.StatePending, ConfirmDuplicate: true})
		code = o.Code()
		return err
	})
	return code, err
}

func (t *taller) Catalog(ctx context.Context, tallerID shared.ID) ([]domain.Priced, []domain.Priced, error) {
	var sv, pt []domain.Priced
	items, err := t.s.M.CatalogF.Active(ctx, tallerID)
	if err != nil {
		return nil, nil, err
	}
	for _, i := range items {
		sv = append(sv, domain.Priced{Name: i.Name, Price: i.Price, Kind: "servicio"})
	}
	parts, err := t.s.M.Inventory.Search(ctx, tallerID, "", false)
	for _, p := range parts {
		price := p.Price
		pt = append(pt, domain.Priced{Name: p.Name, Price: &price, Kind: "repuesto"})
	}
	return sv, pt, err
}

var dayKeys = []string{"dom", "lun", "mar", "mie", "jue", "vie", "sab"}

// Workshop dice si el taller está abierto ahora y cuándo abre la próxima vez (HU-08, HU-61).
func (t *taller) Workshop(ctx context.Context, tallerID shared.ID) (string, bool, string, error) {
	st, err := t.s.M.Admin.Get(ctx, tallerID)
	if err != nil {
		return "", false, "", err
	}
	loc, lerr := time.LoadLocation("America/Bogota")
	if lerr != nil {
		loc = time.FixedZone("COT", -5*3600)
	}
	now := t.s.M.Clock.Now().In(loc)
	hm := now.Format("15:04")
	if len(st.Schedule) == 0 {
		return st.Name, true, "en nuestro horario de atención", nil
	}
	open := false
	if sl := st.Schedule[dayKeys[now.Weekday()]]; sl != nil && hm >= sl.Opens && hm < sl.Closes {
		open = true
	}
	for i := 0; i < 8; i++ {
		d := now.AddDate(0, 0, i)
		sl := st.Schedule[dayKeys[d.Weekday()]]
		if sl == nil || (i == 0 && hm >= sl.Opens) {
			continue
		}
		when := "hoy"
		if i == 1 {
			when = "mañana"
		} else if i > 1 {
			when = "el " + map[string]string{"lun": "lunes", "mar": "martes", "mie": "miércoles", "jue": "jueves", "vie": "viernes", "sab": "sábado", "dom": "domingo"}[dayKeys[d.Weekday()]]
		}
		return st.Name, open, when + " a las " + automationsHour(sl.Opens), nil
	}
	return st.Name, open, "en nuestro próximo horario", nil
}

func automationsHour(hhmm string) string {
	var h, m int
	if _, err := fmtSscanf(hhmm, &h, &m); err != nil {
		return hhmm
	}
	suf := "a. m."
	if h >= 12 {
		suf = "p. m."
	}
	if h > 12 {
		h -= 12
	}
	return itoa(h) + ":" + pad(m) + " " + suf
}
