package wire

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	vehiclesdomain "sweveh/internal/vehicles/domain"
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
	// Lo que el cliente escribió mientras esperaba la autorización: los textos desde la última respuesta a una
	// autorización anterior (gracias / sin autorización) hasta ahora. Se procesan juntos y en orden, para no
	// perder la pregunta original (HU-85) ni crear un borrador por cada "¿hola?".
	var since time.Time
	for _, m := range msgs {
		if m.Direction == "saliente" && m.Automation != nil && *m.Automation == "consentimiento" && m.Kind == "texto" && m.CreatedAt.Before(before) {
			since = m.CreatedAt
		}
	}
	var first *assistantapp.Inbound
	var texts []string
	for _, m := range msgs {
		if m.Direction != "entrante" || m.Kind != "texto" || !m.CreatedAt.After(since) || !m.CreatedAt.Before(before) {
			continue
		}
		if first == nil {
			in, err := t.Message(ctx, tallerID, m.ID)
			if err != nil {
				return nil, err
			}
			first = &in
		}
		texts = append(texts, m.Body)
	}
	if first != nil {
		first.Text = strings.Join(texts, "\n")
	}
	return first, nil
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
			out = append(out, assistantapp.Order{ID: i.ID, Code: i.Code, Plate: i.Plate, Vehicle: i.Vehicle, Kind: i.VehicleKind, State: i.State, PromisedAt: i.PromisedAt})
		}
		return err
	})
	return out, err
}

func (t *taller) Order(ctx context.Context, tallerID, orderID shared.ID) (assistantapp.Order, error) {
	var out assistantapp.Order
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		i, err := t.s.M.OrdersF.Info(ctx, tallerID, orderID)
		out = assistantapp.Order{ID: i.ID, Code: i.Code, Plate: i.Plate, Vehicle: i.Vehicle, Kind: i.VehicleKind, State: i.State, PromisedAt: i.PromisedAt}
		return err
	})
	return out, err
}

func (t *taller) StatusReply(ctx context.Context, tallerID shared.ID, o assistantapp.Order, c assistantapp.Customer) (string, bool, error) {
	return t.s.Automations.StatusReply(ctx, tallerID, automationsapp.OrderInfo{ID: o.ID, Code: o.Code, State: o.State, Plate: o.Plate,
		Vehicle: o.Vehicle, Kind: o.Kind, PromisedAt: o.PromisedAt}, automationsapp.Customer{Name: c.Name, Phone: c.Phone, Consent: c.Consent})
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

// RespondQuoteItems aprueba los ítems con esos números, en el orden en que salieron en el mensaje (HU-30).
func (t *taller) RespondQuoteItems(ctx context.Context, tallerID, orderID shared.ID, picks []int, ref string) (assistantapp.QuotePick, bool, error) {
	var out assistantapp.QuotePick
	q, err := t.s.M.QuotesF.ForOrder(ctx, tallerID, orderID)
	if err != nil || q == nil {
		return out, false, err
	}
	items := make(map[shared.ID]bool, len(q.Lines))
	for _, l := range q.Lines {
		items[l.ID] = false
	}
	for _, n := range picks {
		if n > len(q.Lines) {
			return out, false, nil
		}
		items[q.Lines[n-1].ID] = true
	}
	res, err := t.s.M.QuotesF.RespondItems(ctx, tallerID, orderID, items, "whatsapp", ref, agent)
	if err != nil {
		return out, false, err
	}
	return assistantapp.QuotePick{Approved: len(picks), Lines: len(q.Lines), Total: res.Total}, true, nil
}

func (t *taller) AnswerReceipt(ctx context.Context, tallerID, orderID shared.ID, ok bool, ref string) (bool, error) {
	return t.s.M.OrdersF.AnswerReceipt(ctx, tallerID, orderID, agent, ok, ref)
}

// CreateOrderFromPlate crea vehículo (motor pendiente) y orden "Por recibir" (HU-55, HU-91).
func (t *taller) PlateOf(ctx context.Context, tallerID, customerID shared.ID, plate string) (assistantapp.PlateInfo, error) {
	var out assistantapp.PlateInfo
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = t.plateOf(ctx, tallerID, customerID, plate)
		return err
	})
	return out, err
}

// plateOf corre en la transacción del llamador.
func (t *taller) plateOf(ctx context.Context, tallerID, customerID shared.ID, plate string) (assistantapp.PlateInfo, error) {
	v, err := t.s.M.VehDir.ByPlate(ctx, tallerID, plate)
	switch {
	case errors.Is(err, vehiclesdomain.ErrVehicleNotFound):
		return assistantapp.PlateInfo{Owner: "nueva"}, nil
	case err != nil:
		return assistantapp.PlateInfo{}, err
	case v.CustomerID != customerID:
		return assistantapp.PlateInfo{Owner: "ajena"}, nil
	}
	open, err := t.s.M.OrdersF.OpenByCustomer(ctx, tallerID, customerID, []shared.ID{v.ID})
	if err != nil || len(open) == 0 {
		return assistantapp.PlateInfo{Owner: "propia"}, err
	}
	i := open[0]
	return assistantapp.PlateInfo{Owner: "propia", Code: i.Code, Open: &assistantapp.Order{ID: i.ID, Code: i.Code, Plate: i.Plate,
		Vehicle: i.Vehicle, Kind: i.VehicleKind, State: i.State, PromisedAt: i.PromisedAt}}, nil
}

func (t *taller) CreateOrderFromPlate(ctx context.Context, tallerID, customerID shared.ID, plate, symptom string, when *time.Time) (assistantapp.PlateInfo, error) {
	var out assistantapp.PlateInfo
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		p, err := t.plateOf(ctx, tallerID, customerID, plate)
		if err != nil || p.Owner == "ajena" {
			out = p
			return err
		}
		if p.Open != nil { // con orden abierta: esa misma; si aún no llega el vehículo, se cambia la cita
			out = p
			if when == nil || p.Open.State != string(ordersdomain.StatePending) {
				return nil
			}
			out.Rescheduled = true
			_, err := t.s.M.Flow.SetPromise(ctx, tallerID, p.Open.ID, agent, when)
			return err
		}
		v, err := t.s.M.Vehicles.Register(ctx, vehiclesapp.RegisterVehicle{TallerID: tallerID, Actor: agent, Plate: plate, CustomerID: customerID})
		if err != nil {
			return err
		}
		o, err := t.s.M.Orders.Open(ctx, ordersapp.OpenOrder{TallerID: tallerID, Actor: agent, VehicleID: v.Vehicle.ID, Symptom: symptom,
			InitialState: ordersdomain.StatePending})
		if err != nil {
			return err
		}
		out = assistantapp.PlateInfo{Owner: p.Owner, Code: o.Code()}
		if when != nil { // la cita queda en la fecha prometida: AU-06 la recuerda ese día
			_, err = t.s.M.Flow.SetPromise(ctx, tallerID, o.ID, agent, when)
		}
		return err
	})
	return out, err
}

// OpenAt revisa la hora contra el horario del taller (HU-08). Sin horario registrado se acepta y el taller confirma.
func (t *taller) OpenAt(ctx context.Context, tallerID shared.ID, at time.Time) (bool, string, error) {
	st, err := t.s.M.Admin.Get(ctx, tallerID)
	if err != nil {
		return false, "", err
	}
	if len(st.Schedule) == 0 {
		return true, "", nil
	}
	at = at.In(domain.Bogota)
	sl := st.Schedule[dayKeys[at.Weekday()]]
	if sl == nil {
		return false, "", nil
	}
	hm := at.Format("15:04")
	return hm >= sl.Opens && hm < sl.Closes, automationsHour(sl.Opens) + " a " + automationsHour(sl.Closes), nil
}

func (t *taller) SetCustomerName(ctx context.Context, tallerID, customerID shared.ID, name string) (bool, error) {
	return t.s.M.CustDir.FillName(ctx, tallerID, customerID, name, agent)
}

func (t *taller) Catalog(ctx context.Context, tallerID shared.ID) ([]domain.Priced, []domain.Priced, error) {
	var sv, pt []domain.Priced
	items, err := t.s.M.CatalogF.Active(ctx, tallerID)
	if err != nil {
		return nil, nil, err
	}
	for _, i := range items {
		sv = append(sv, domain.Priced{Name: i.Name, Price: i.Price, Kind: "servicio", Vehicle: i.Kind})
	}
	parts, err := t.s.M.Inventory.Search(ctx, tallerID, "", false, "")
	for _, p := range parts {
		price := p.Price
		pt = append(pt, domain.Priced{Name: p.Name, Price: &price, Kind: "repuesto", Vehicle: p.Kind})
	}
	return sv, pt, err
}

// Attends es lo que atiende el taller según su perfil (carro, moto o ambos; vacío = ambos).
func (t *taller) Attends(ctx context.Context, tallerID shared.ID) (string, error) {
	st, err := t.s.M.Admin.Get(ctx, tallerID)
	if err != nil || st.Profile.Attends == "" {
		return "ambos", err
	}
	return st.Profile.Attends, nil
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
