package wire

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	assistantapp "sweveh/internal/assistant/app"
	assistantdomain "sweveh/internal/assistant/domain"
	customersapp "sweveh/internal/customers/app"
	customersdomain "sweveh/internal/customers/domain"
	ordersapp "sweveh/internal/orders/app"
	ordersdomain "sweveh/internal/orders/domain"
	"sweveh/internal/platform/storage"
	quotesapp "sweveh/internal/quotes/app"
	"sweveh/internal/server"
	"sweveh/internal/shared"
	vehiclesapp "sweveh/internal/vehicles/app"
	vehiclesdomain "sweveh/internal/vehicles/domain"
)

// floorShop conecta el asistente del piso (R8, ADR-0041) con los módulos, cada cosa por el caso de uso que ya existe:
// así la orden, la ficha, la recepción, los hallazgos, la cotización y los gastos que salen del asistente pasan por las
// mismas reglas, bitácora y eventos que cuando se hacen a mano.
type floorShop struct{ m *server.Modules }

var _ assistantapp.FloorWorkshop = floorShop{}

func (f floorShop) Context(ctx context.Context, tallerID shared.ID) (assistantapp.FloorContext, error) {
	st, err := f.m.Admin.Get(ctx, tallerID)
	if err != nil {
		return assistantapp.FloorContext{}, err
	}
	out := assistantapp.FloorContext{Workshop: st.Name, Attends: st.Profile.Attends}
	if out.Attends == "" {
		out.Attends = "ambos"
	}
	items, err := f.m.CatalogF.Active(ctx, tallerID)
	if err != nil {
		return out, err
	}
	for _, i := range items {
		out.Services = append(out.Services, assistantdomain.ListService{ID: i.ID.String(), Name: i.Name, Price: i.Price, Kind: i.Kind})
	}
	p, err := f.m.Admin.Parametros(ctx, tallerID)
	if err != nil {
		return out, err
	}
	for _, it := range p.CarItems {
		out.ItemsCar = append(out.ItemsCar, assistantapp.FloorChecklistItem{Key: it.Key, Name: it.Name})
	}
	for _, it := range p.MotoItems {
		out.ItemsMoto = append(out.ItemsMoto, assistantapp.FloorChecklistItem{Key: it.Key, Name: it.Name})
	}
	return out, nil
}

func describe(brand, line *string, year *int) string {
	var parts []string
	if brand != nil && *brand != "" {
		parts = append(parts, *brand)
	}
	if line != nil && *line != "" {
		parts = append(parts, *line)
	}
	if year != nil && *year > 0 {
		parts = append(parts, strconv.Itoa(*year))
	}
	return strings.Join(parts, " ")
}

func (f floorShop) Vehicle(ctx context.Context, tallerID shared.ID, plate string) (*assistantapp.FloorVehicle, error) {
	v, err := f.m.Vehicles.ByPlate(ctx, tallerID, plate)
	if errors.Is(err, vehiclesdomain.ErrVehicleNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &assistantapp.FloorVehicle{ID: v.ID.String(), Plate: v.Plate.String(), Kind: string(v.Kind),
		Description: describe(v.Details.Brand, v.Details.Line, v.Details.Year)}
	if c, err := f.m.Customers.ByID(ctx, tallerID, v.CustomerID); err == nil {
		out.CustomerName = c.Name
	}
	open, err := f.m.OrdersF.OpenByVehicle(ctx, tallerID, v.ID)
	if err != nil {
		return nil, err
	}
	if len(open) > 0 {
		o := open[0]
		out.Open = &assistantapp.FloorOrder{ID: o.ID.String(), Code: o.Code, State: o.State, Plate: o.Plate, Kind: o.VehicleKind,
			Vehicle: out.Description}
	}
	return out, nil
}

func (f floorShop) CustomerByPhone(ctx context.Context, tallerID shared.ID, phone string) (*assistantapp.FloorPerson, error) {
	c, err := f.m.Customers.ByPhone(ctx, tallerID, phone)
	if errors.Is(err, customersdomain.ErrCustomerNotFound) || errors.Is(err, shared.ErrPhoneInvalid) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &assistantapp.FloorPerson{ID: c.ID.String(), Name: c.Name}, nil
}

func (f floorShop) Order(ctx context.Context, tallerID, orderID shared.ID) (assistantapp.FloorOrder, error) {
	v, err := f.m.Flow.Get(ctx, tallerID, orderID)
	if err != nil {
		return assistantapp.FloorOrder{}, err
	}
	o := assistantapp.FloorOrder{ID: v.Order.ID.String(), Code: v.Order.Code(), State: string(v.Order.State), Plate: v.Order.Plate,
		Kind: v.Order.VehicleKind}
	if v.Vehicle != nil {
		o.Vehicle = describe(v.Vehicle.Brand, v.Vehicle.Line, v.Vehicle.Year)
	}
	return o, nil
}

func (f floorShop) OrderStatus(ctx context.Context, tallerID, orderID shared.ID) (assistantapp.FloorOrderStatus, error) {
	o, err := f.m.Flow.Load(ctx, tallerID, orderID)
	if err != nil {
		return assistantapp.FloorOrderStatus{}, err
	}
	st := assistantapp.FloorOrderStatus{Code: o.Code(), State: string(o.State)}
	if o.State == ordersdomain.StatePending {
		cv, err := f.m.Flow.GetChecklist(ctx, tallerID, orderID)
		if err != nil {
			return st, err
		}
		st.Missing = cv.Missing
	}
	q, err := f.m.QuotesF.ForOrder(ctx, tallerID, orderID)
	if err != nil || q == nil {
		return st, err
	}
	st.QuoteState, st.QuoteItems, st.QuoteTotal = q.State, len(q.Lines), q.Total
	return st, nil
}

func (f floorShop) Saved(ctx context.Context, tallerID, orderID shared.ID) (assistantdomain.Saved, error) {
	cv, err := f.m.Flow.GetChecklist(ctx, tallerID, orderID)
	if err != nil {
		return assistantdomain.Saved{}, err
	}
	o, err := f.m.Flow.Load(ctx, tallerID, orderID)
	return assistantdomain.Saved{Sides: cv.Sides, Symptom: strings.TrimSpace(o.Symptom) != "", Km: cv.Checklist.Km != nil,
		Fuel: cv.Checklist.Fuel != nil}, err
}

// Apply corre dentro de la transacción del Confirmar. El orden importa: la orden, la ficha, las fotos y la recepción
// van antes del cambio de estado (pasar a Recibido las exige) y el estado antes de los hallazgos y la cotización
// (que piden la orden en diagnóstico o en reparación).
func (f floorShop) Apply(ctx context.Context, tallerID shared.ID, actor shared.Actor, p assistantdomain.FloorProposal,
	photo func(ctx context.Context, ref string) ([]byte, error)) (assistantapp.FloorApplied, error) {
	var out assistantapp.FloorApplied
	orderID, vehicleID, err := f.order(ctx, tallerID, actor, p, &out)
	if err != nil {
		return out, err
	}
	if err := f.sheet(ctx, tallerID, actor, vehicleID, p, &out); err != nil {
		return out, err
	}
	for _, ph := range p.Photos {
		data, err := photo(ctx, ph.Ref)
		if err != nil {
			return out, err
		}
		var side *string
		if ph.Side != "" {
			s := ph.Side
			side = &s
		}
		if _, err := f.m.Flow.AddEvidence(ctx, ordersapp.EvidenceInput{TallerID: tallerID, OrderID: orderID, Actor: actor, Side: side,
			Photo: bytes.NewReader(data)}); err != nil {
			return out, err
		}
		out.Photos++
	}
	if out.Photos > 0 {
		out.Done = append(out.Done, plural(out.Photos, "foto", "fotos"))
	}
	if err := f.reception(ctx, tallerID, actor, orderID, p, &out); err != nil {
		return out, err
	}
	if p.Move != nil {
		if _, err := f.m.Flow.Transition(ctx, ordersapp.TransitionCmd{TallerID: tallerID, OrderID: orderID, Actor: actor,
			To: ordersdomain.State(p.Move.Value)}); err != nil {
			return out, err
		}
		out.Done = append(out.Done, "el cambio de estado")
	}
	if err := f.findings(ctx, tallerID, actor, orderID, p, &out); err != nil {
		return out, err
	}
	if err := f.quote(ctx, tallerID, actor, orderID, p.Quote, &out); err != nil {
		return out, err
	}
	for _, e := range p.Expenses {
		id, err := shared.ParseID(e.ID)
		if err != nil {
			id = shared.NewID()
		}
		origin := ordersdomain.ExpenseAssistant
		if e.Origin == assistantdomain.FromPhoto {
			origin = ordersdomain.ExpenseInvoice
		}
		if _, _, err := f.m.Expenses.Add(ctx, ordersapp.ExpenseInput{TallerID: tallerID, OrderID: orderID, ID: id, Actor: actor,
			Description: e.Description, Value: e.Value, Origin: origin}); err != nil {
			return out, err
		}
	}
	if n := len(p.Expenses); n > 0 {
		out.Done = append(out.Done, plural(n, "gasto", "gastos"))
	}
	o, err := f.m.Flow.Load(ctx, tallerID, orderID)
	if err != nil {
		return out, err
	}
	out.OrderID, out.OrderCode, out.OrderState = o.ID, o.Code(), string(o.State)
	return out, nil
}

// findings guarda los hallazgos si la orden ya los admite (desde Recibido hasta En reparación). Si todavía no (la
// recepción por video de una orden Por recibir), se guarda lo demás y los hallazgos quedan en la ficha (out.Held)
// para cuando la orden pase a Recibido: guardar por partes no pierde lo que dijo el mecánico. Si solo había
// hallazgos, se responde la regla de la orden, para que la persona sepa por qué no se guardó nada.
func (f floorShop) findings(ctx context.Context, tallerID shared.ID, actor shared.Actor, orderID shared.ID, p assistantdomain.FloorProposal,
	out *assistantapp.FloorApplied) error {
	if len(p.Findings) == 0 {
		return nil
	}
	o, err := f.m.Flow.Load(ctx, tallerID, orderID)
	if err != nil {
		return err
	}
	if !o.CanAddFindings() {
		if len(out.Done) == 0 && len(p.Quote) == 0 && len(p.Expenses) == 0 {
			return ordersdomain.ErrFindingState
		}
		out.Held = p.Findings
		return nil
	}
	for _, fd := range p.Findings {
		if _, err := f.m.Findings.Add(ctx, ordersapp.FindingInput{TallerID: tallerID, OrderID: orderID, Actor: actor, System: fd.System,
			Description: fd.Description, Severity: fd.Severity}); err != nil {
			return err
		}
	}
	out.Done = append(out.Done, plural(len(p.Findings), "hallazgo", "hallazgos"))
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// order abre la orden (con cliente y vehículo si son nuevos) o usa la que ya estaba. Nunca duplica una orden
// abierta: Open la rechaza si el vehículo ya tiene una.
func (f floorShop) order(ctx context.Context, tallerID shared.ID, actor shared.Actor, p assistantdomain.FloorProposal,
	out *assistantapp.FloorApplied) (orderID, vehicleID shared.ID, err error) {
	if !p.NewOrder {
		if orderID, err = shared.ParseID(p.OrderID); err != nil {
			return orderID, vehicleID, assistantdomain.ErrFloorNeedsOrder
		}
		o, err := f.m.Flow.Load(ctx, tallerID, orderID)
		return orderID, o.VehicleID, err
	}
	var v vehiclesdomain.Vehicle
	if p.VehicleNew {
		customerID, err := f.customer(ctx, tallerID, actor, p.Customer)
		if err != nil {
			return orderID, vehicleID, err
		}
		details, engine := sheetFrom(p.Sheet, vehiclesdomain.Details{}, vehiclesdomain.Engine{})
		if p.Km != nil {
			if n, err := strconv.Atoi(p.Km.Value); err == nil {
				details.Mileage = &n
			}
		}
		res, err := f.m.Vehicles.Register(ctx, vehiclesapp.RegisterVehicle{TallerID: tallerID, Actor: actor, Plate: p.Plate.Value, Kind: p.Kind,
			CustomerID: customerID, Details: details, Engine: engine})
		if err != nil {
			return orderID, vehicleID, err
		}
		v = res.Vehicle
		out.Done = append(out.Done, "el vehículo "+v.Plate.String())
	} else if v, err = f.m.Vehicles.ByPlate(ctx, tallerID, p.Plate.Value); err != nil {
		return orderID, vehicleID, err
	}
	symptom := ""
	if p.Symptom != nil {
		symptom = p.Symptom.Value
	}
	o, err := f.m.Orders.Open(ctx, ordersapp.OpenOrder{TallerID: tallerID, Actor: actor, VehicleID: v.ID, Symptom: symptom,
		InitialState: ordersdomain.StatePending})
	if err != nil {
		return orderID, vehicleID, err
	}
	out.Created = true
	out.Done = append(out.Done, "la orden "+o.Code())
	return o.ID, v.ID, nil
}

// customer usa el cliente que ya existe por su celular o lo registra (HU-09).
func (f floorShop) customer(ctx context.Context, tallerID shared.ID, actor shared.Actor, c *assistantdomain.FloorCustomer) (shared.ID, error) {
	if c == nil {
		return shared.NilID, assistantdomain.ErrFloorCustomer
	}
	if c.ID != "" {
		if id, err := shared.ParseID(c.ID); err == nil {
			return id, nil
		}
	}
	if c.Phone == nil || c.Name == nil {
		return shared.NilID, assistantdomain.ErrFloorCustomer
	}
	// HU-86 (R11): un cliente nuevo no se guarda sin su respuesta a la autorización, que marcó la persona en la ficha.
	cmd := customersapp.RegisterCustomer{TallerID: tallerID, Actor: actor, Phone: c.Phone.Value, Name: c.Name.Value, RequireConsent: true}
	if c.Consent != nil {
		by := actor.ID
		cmd.Consent = &customersapp.ConsentAnswer{Granted: *c.Consent, Record: customersapp.ConsentRecord{
			Text: customersdomain.CounterConsentText, Version: customersdomain.CounterConsentVersion, Channel: "mostrador", By: &by}}
	}
	res, err := f.m.Customers.Register(ctx, cmd)
	return res.Customer.ID, err
}

// sheetFrom pasa la ficha de la propuesta a los datos del vehículo, sobre lo que ya tenía.
func sheetFrom(s map[string]assistantdomain.Field, d vehiclesdomain.Details, e vehiclesdomain.Engine) (vehiclesdomain.Details, vehiclesdomain.Engine) {
	str := func(k string) *string {
		if v, ok := s[k]; ok && strings.TrimSpace(v.Value) != "" {
			x := strings.TrimSpace(v.Value)
			return &x
		}
		return nil
	}
	num := func(k string) *int {
		if v := str(k); v != nil {
			if n, err := strconv.Atoi(*v); err == nil {
				return &n
			}
		}
		return nil
	}
	pick := func(cur, next *string) *string {
		if next != nil {
			return next
		}
		return cur
	}
	pickN := func(cur, next *int) *int {
		if next != nil {
			return next
		}
		return cur
	}
	d.Brand, d.Line, d.Year, d.Color = pick(d.Brand, str("marca")), pick(d.Line, str("linea")), pickN(d.Year, num("anio")), pick(d.Color, str("color"))
	e.DisplacementCC = pickN(e.DisplacementCC, num("cilindraje"))
	e.Energy, e.Aspiration, e.Transmission = pick(e.Energy, str("combustible")), pick(e.Aspiration, str("aspiracion")), pick(e.Transmission, str("caja"))
	e.Drive, e.Cycle, e.Cooling = pick(e.Drive, str("traccion")), pick(e.Cycle, str("ciclo")), pick(e.Cooling, str("refrigeracion"))
	return d, e
}

// sheet completa la ficha de un vehículo que ya estaba. Marca, línea y año no se cambian desde aquí (UpdateEngine
// solo cambia el motor, el color, la carrocería y la llanta).
func (f floorShop) sheet(ctx context.Context, tallerID shared.ID, actor shared.Actor, vehicleID shared.ID, p assistantdomain.FloorProposal,
	out *assistantapp.FloorApplied) error {
	if len(p.Sheet) == 0 {
		return nil
	}
	if p.VehicleNew {
		out.Done = append(out.Done, "la ficha")
		return nil // ya entró con el registro del vehículo
	}
	v, err := f.m.Vehicles.ByID(ctx, tallerID, vehicleID)
	if err != nil {
		return err
	}
	d, e := sheetFrom(p.Sheet, v.Details, v.Engine)
	if _, err := f.m.Vehicles.UpdateEngine(ctx, vehiclesapp.UpdateEngine{TallerID: tallerID, Actor: actor, VehicleID: v.ID, Engine: e,
		Details: vehiclesdomain.Details{Color: d.Color, Body: v.Details.Body, Tire: v.Details.Tire}, ExpectedVersion: v.Version}); err != nil {
		return err
	}
	out.Done = append(out.Done, "la ficha")
	return nil
}

func (f floorShop) reception(ctx context.Context, tallerID shared.ID, actor shared.Actor, orderID shared.ID, p assistantdomain.FloorProposal,
	out *assistantapp.FloorApplied) error {
	if p.Km == nil && p.Fuel == nil && len(p.Items) == 0 && len(p.Notes) == 0 {
		return nil
	}
	cv, err := f.m.Flow.GetChecklist(ctx, tallerID, orderID)
	if err != nil {
		return err
	}
	cl := cv.Checklist
	if p.Km != nil {
		n, err := strconv.Atoi(p.Km.Value)
		if err != nil {
			return assistantdomain.ErrFloorKm
		}
		cl.Km = &n
	}
	if p.Fuel != nil {
		fuel := p.Fuel.Value
		cl.Fuel = &fuel
	}
	for _, it := range p.Items {
		found := false
		for i := range cl.Items {
			if cl.Items[i].Key == it.Key {
				cl.Items[i].State, cl.Items[i].Note, found = it.State, it.Note, true
			}
		}
		if !found {
			cl.Items = append(cl.Items, ordersdomain.ItemCheck{Key: it.Key, Name: it.Name, State: it.State, Note: it.Note})
		}
	}
	for _, n := range p.Notes {
		cl.Exceptions = append(cl.Exceptions, ordersdomain.Exception{Note: n.Value})
	}
	if _, err := f.m.Flow.SaveChecklist(ctx, tallerID, orderID, actor, cl); err != nil {
		return err
	}
	out.Done = append(out.Done, "la recepción")
	return nil
}

func (f floorShop) quote(ctx context.Context, tallerID shared.ID, actor shared.Actor, orderID shared.ID, lines []assistantdomain.FloorLine,
	out *assistantapp.FloorApplied) error {
	for _, l := range lines {
		in := quotesapp.LineInput{Kind: l.Kind, Description: l.Name, Qty: l.Qty, UnitPrice: l.Price}
		switch l.Kind {
		case "mano_obra":
			in.Kind = "servicio" // mano de obra escrita: un ítem libre con el valor que dijo la persona
		case "servicio":
			if id, err := shared.ParseID(l.ServiceID); err == nil && l.ServiceID != "" {
				in.RefID = &id
				if l.PriceOrigin == assistantdomain.FromList {
					in.UnitPrice = nil // manda la lista del taller
				}
			}
		}
		if _, err := f.m.Quotes.AddLine(ctx, tallerID, orderID, actor, in); err != nil {
			return err
		}
	}
	if n := len(lines); n > 0 {
		out.Done = append(out.Done, plural(n, "ítem en la cotización", "ítems en la cotización"))
	}
	return nil
}

// floorFiles guarda las fotos del chat en el almacenamiento del api (claves taller/uuid).
type floorFiles struct{ st storage.Store }

func (f floorFiles) Put(ctx context.Context, key string, data []byte) error {
	return f.st.Put(ctx, key, bytes.NewReader(data), assistantapp.FloorMaxPhoto)
}

func (f floorFiles) Get(ctx context.Context, key string) ([]byte, error) {
	r, err := f.st.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(io.LimitReader(r, assistantapp.FloorMaxPhoto+1))
}
