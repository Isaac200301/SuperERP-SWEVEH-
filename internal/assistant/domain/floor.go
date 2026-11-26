package domain

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"sweveh/internal/shared"
)

// Asistente del piso (R8, ADR-0041, HU-107): el mecánico le habla, le escribe o le manda fotos y el asistente arma una
// propuesta (orden, ficha, recepción, hallazgos, cotización, gastos y estado). La propuesta no se guarda en la orden
// hasta que una persona toca Confirmar (R-08). Este archivo tiene las reglas que no dependen de la IA: de dónde puede
// salir un precio, qué falta para recibir y qué se exige antes de confirmar.

// Orígenes de un dato de la propuesta (HU-64): la ficha en vivo dice de dónde salió cada uno.
const (
	FromSaid     = "dijiste" // lo dijo o lo escribió la persona en esta conversación
	FromPhoto    = "foto"    // lo leyó el modelo de una foto que mandó la persona
	FromList     = "lista"   // la lista de servicios del taller
	FromYou      = "pusiste" // la persona lo escribió en la ficha en vivo
	FromWorkshop = "taller"  // ya estaba en el sistema (vehículo, cliente u orden registrados)
)

// Field es un dato con su origen.
type Field struct {
	Value  string `json:"valor"`
	Origin string `json:"origen"`
}

// FloorCustomer es el dueño de un vehículo nuevo. ID viene lleno si el celular ya es de un cliente del taller.
type FloorCustomer struct {
	ID    string `json:"id,omitempty"`
	Phone *Field `json:"celular,omitempty"`
	Name  *Field `json:"nombre,omitempty"`
}

// FloorItem es un ítem del checklist de recepción del taller.
type FloorItem struct {
	Key    string `json:"clave"`
	Name   string `json:"nombre"`
	State  string `json:"estado"`
	Note   string `json:"nota,omitempty"`
	Origin string `json:"origen"`
}

// FloorPhoto es una foto que mandó la persona; Side es el lado del vehículo que muestra ("" = no se sabe).
type FloorPhoto struct {
	Ref    string `json:"ref"`
	Side   string `json:"lado,omitempty"`
	Origin string `json:"origen"`
}

// FloorFinding es un hallazgo que dijo o mostró el mecánico. La IA no diagnostica por su cuenta.
type FloorFinding struct {
	ID          string `json:"id"`
	System      string `json:"sistema"`
	Description string `json:"descripcion"`
	Severity    string `json:"gravedad"`
	Origin      string `json:"origen"`
}

// FloorLine es un ítem de la cotización. Price nil = "Sin precio · ponlo": nunca se sugiere un valor (RF-17).
type FloorLine struct {
	ID          string `json:"id"`
	Kind        string `json:"tipo"` // servicio | partida | repuesto | mano_obra
	Name        string `json:"nombre"`
	Qty         int    `json:"cantidad"`
	Price       *int64 `json:"precio"`
	PriceOrigin string `json:"origenPrecio,omitempty"`
	ServiceID   string `json:"servicioId,omitempty"`
}

// FloorExpense es un gasto de la obra (R6). El valor sale de lo que dijo la persona o de la factura que mostró.
type FloorExpense struct {
	ID          string `json:"id"`
	Description string `json:"descripcion"`
	Value       int64  `json:"valor"`
	Origin      string `json:"origen"`
}

// FloorProposal es todo lo que el asistente propone en una conversación y aún no está guardado.
type FloorProposal struct {
	Plate      *Field           `json:"placa,omitempty"`
	Kind       string           `json:"tipo,omitempty"` // carro | moto
	Vehicle    string           `json:"vehiculo,omitempty"`
	VehicleNew bool             `json:"vehiculoNuevo,omitempty"`
	OrderID    string           `json:"ordenId,omitempty"`
	OrderCode  string           `json:"ordenCodigo,omitempty"`
	OrderState string           `json:"ordenEstado,omitempty"`
	NewOrder   bool             `json:"nuevaOrden,omitempty"`
	Customer   *FloorCustomer   `json:"cliente,omitempty"`
	Symptom    *Field           `json:"sintoma,omitempty"`
	Sheet      map[string]Field `json:"ficha,omitempty"`
	Km         *Field           `json:"kilometraje,omitempty"`
	Fuel       *Field           `json:"combustible,omitempty"`
	Items      []FloorItem      `json:"items,omitempty"`
	Notes      []Field          `json:"notas,omitempty"`
	Photos     []FloorPhoto     `json:"fotos,omitempty"`
	Findings   []FloorFinding   `json:"hallazgos,omitempty"`
	Quote      []FloorLine      `json:"cotizacion,omitempty"`
	Expenses   []FloorExpense   `json:"gastos,omitempty"`
	Move       *Field           `json:"estado,omitempty"`
}

// Pending dice si hay algo por confirmar.
func (p FloorProposal) Pending() bool {
	return p.NewOrder || p.Symptom != nil || len(p.Sheet) > 0 || p.Km != nil || p.Fuel != nil || len(p.Items) > 0 ||
		len(p.Notes) > 0 || len(p.Photos) > 0 || len(p.Findings) > 0 || len(p.Quote) > 0 || len(p.Expenses) > 0 || p.Move != nil
}

// Reset deja la propuesta vacía después de confirmar, con la placa y la orden para seguir la conversación.
func (p FloorProposal) Reset(orderID, code, state string) FloorProposal {
	return FloorProposal{Plate: p.Plate, Kind: p.Kind, Vehicle: p.Vehicle, OrderID: orderID, OrderCode: code, OrderState: state}
}

// ---- De dónde puede salir un precio ----

var (
	reAmount = regexp.MustCompile(`(?i)\$?\s?([0-9]{1,3}(?:[.,][0-9]{3})+|[0-9]+)(\s?(?:mil|k)\b)?`)
)

// SaidAmounts son las cifras que la persona dijo o escribió literalmente ("$120.000", "120000", "120 mil"). Son las
// únicas, junto con la lista del taller, de donde puede salir un precio o el valor de un gasto (RF-17).
func SaidAmounts(texts ...string) map[int64]bool {
	out := map[int64]bool{}
	for _, t := range texts {
		for _, m := range reAmount.FindAllStringSubmatch(t, -1) {
			digits := strings.NewReplacer(".", "", ",", "").Replace(m[1])
			n, err := strconv.ParseInt(digits, 10, 64)
			if err != nil || n <= 0 {
				continue
			}
			if m[2] != "" {
				n *= 1000
			}
			out[n] = true
		}
	}
	return out
}

// ListService es un servicio de la lista del taller.
type ListService struct {
	ID    string
	Name  string
	Price *int64
	Kind  string // carro | moto | ambos
}

// PriceLine fija el precio de un ítem que propuso la IA. Un servicio de la lista toma el precio de la lista; si la
// lista no lo tiene, o es otro tipo de ítem, el precio solo vale si la persona lo dijo. Lo demás queda sin precio.
// dropped dice si se descartó un precio que propuso la IA.
func PriceLine(l FloorLine, list []ListService, said map[int64]bool) (out FloorLine, dropped bool) {
	out = l
	proposed := l.Price
	out.Price, out.PriceOrigin = nil, ""
	if out.Kind == "servicio" {
		if s, ok := FindService(list, l.ServiceID, l.Name); ok {
			out.ServiceID, out.Name = s.ID, s.Name
			if s.Price != nil {
				p := *s.Price
				out.Price, out.PriceOrigin = &p, FromList
				return out, proposed != nil && *proposed != p
			}
		} else {
			out.ServiceID = ""
		}
	}
	if proposed != nil && *proposed > 0 && said[*proposed] {
		p := *proposed
		out.Price, out.PriceOrigin = &p, FromSaid
		return out, false
	}
	return out, proposed != nil
}

// FindService busca el servicio por id o, si no, por nombre sin tildes ni mayúsculas.
func FindService(list []ListService, id, name string) (ListService, bool) {
	if id != "" {
		for _, s := range list {
			if s.ID == id {
				return s, true
			}
		}
	}
	n := strings.TrimSpace(Normalize(name))
	if n == "" {
		return ListService{}, false
	}
	for _, s := range list {
		if strings.TrimSpace(Normalize(s.Name)) == n {
			return s, true
		}
	}
	return ListService{}, false
}

// reTextAmount son las cifras de plata que puede escribir el asistente: con $, "95 mil" o "95.000" (con punto de
// miles). Lo que va seguido de km o kilómetros es kilometraje y no cuenta.
var reTextAmount = regexp.MustCompile(`(?i)(\$\s?)?([0-9]{1,3}(?:\.[0-9]{3})+|[0-9]+)(\s?mil\b)?(\s?(?:km|kil[oó]metros)\b)?`)

// FloorTextOK revisa el texto que el asistente le muestra a la persona: cada cifra de plata (con $, "95 mil" o
// "95.000") tiene que estar en la lista del taller, en lo que dijo la persona o en la propuesta (que ya pasó por
// PriceLine).
func FloorTextOK(text string, allowed map[int64]bool) bool {
	for _, m := range reTextAmount.FindAllStringSubmatch(text, -1) {
		money := m[1] != "" || m[3] != "" || strings.Contains(m[2], ".")
		if !money || m[4] != "" {
			continue
		}
		n, err := strconv.ParseInt(strings.ReplaceAll(m[2], ".", ""), 10, 64)
		if m[3] != "" {
			n *= 1000
		}
		if err != nil || !allowed[n] {
			return false
		}
	}
	return true
}

// ---- Placas ----

// FindPlates devuelve las placas distintas del texto, en orden.
func FindPlates(text string) []string {
	var out []string
	for _, m := range rePlate.FindAllStringSubmatch(text, -1) {
		p := strings.ToUpper(m[1] + m[2] + m[3])
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// ---- Qué falta para recibir ("7 de 8 datos · falta la parte trasera") ----

// Progress es el resumen de la ficha en vivo.
type Progress struct {
	Have    int      `json:"tiene"`
	Total   int      `json:"total"`
	Missing []string `json:"falta"`
}

var sideWords = map[string]string{"frente": "el frente", "trasera": "la parte trasera", "izquierdo": "el lado izquierdo",
	"derecho": "el lado derecho", "tablero": "el tablero"}

// CarSides y MotoSides son los lados con foto de la recepción (HU-17 y HU-18).
var (
	CarSides   = []string{"frente", "trasera", "izquierdo", "derecho"}
	MotoSides  = []string{"frente", "trasera", "izquierdo", "derecho", "tablero"}
	Fuels      = []string{"vacio", "1/4", "1/2", "3/4", "lleno", "no_aplica"}
	Systems    = []string{"motor", "frenos", "suspension", "direccion", "electrico", "transmision", "refrigeracion", "llantas", "carroceria", "otro"}
	Severities = []string{"urgente", "recomendado", "puede_esperar"}
	States     = []string{"recibido", "en_diagnostico", "esperando_aprobacion", "en_reparacion", "esperando_repuesto", "control_calidad", "listo"}
)

// Receiving dice si la conversación está recibiendo el vehículo (orden nueva o "Por recibir").
func (p FloorProposal) Receiving() bool {
	return p.NewOrder || p.OrderState == "por_recibir" || (p.OrderID == "" && p.Plate != nil)
}

// Saved es lo que la orden ya tiene guardado de la recepción: cuenta como hecho en la ficha en vivo.
type Saved struct {
	Sides             []string
	Symptom, Km, Fuel bool
}

// Progress cuenta los datos de la recepción: placa, cliente, qué trae, kilometraje, combustible y una foto por lado,
// con lo que propone la conversación o ya está guardado en la orden. Si no se está recibiendo, no cuenta nada.
func (p FloorProposal) Progress(saved Saved) Progress {
	if !p.Receiving() {
		return Progress{}
	}
	var pr Progress
	check := func(ok bool, missing string) {
		pr.Total++
		if ok {
			pr.Have++
		} else {
			pr.Missing = append(pr.Missing, missing)
		}
	}
	check(p.Plate != nil && p.Plate.Value != "", "la placa")
	if p.VehicleNew {
		check(p.Customer != nil && p.Customer.Phone != nil && p.Customer.Name != nil, "el celular y el nombre del cliente")
	}
	check(saved.Symptom || (p.Symptom != nil && strings.TrimSpace(p.Symptom.Value) != ""), "qué trae")
	check(saved.Km || p.Km != nil, "el kilometraje")
	check(saved.Fuel || p.Fuel != nil, "el combustible")
	need := CarSides
	if p.Kind == "moto" {
		need = MotoSides
	}
	for _, s := range need {
		ok := slices.Contains(saved.Sides, s)
		for _, ph := range p.Photos {
			ok = ok || ph.Side == s
		}
		check(ok, sideWords[s])
	}
	return pr
}

// ---- Antes de confirmar ----

var (
	ErrFloorEmpty      = shared.Validation("propuesta_vacia", "No hay nada por confirmar todavía.")
	ErrFloorPlate      = shared.Validation("placa_requerida", "Dime la placa del vehículo antes de confirmar.").WithField("placa")
	ErrFloorCustomer   = shared.Validation("cliente_requerido", "El vehículo es nuevo: falta el celular y el nombre del cliente.").WithField("cliente")
	ErrFloorKind       = shared.Validation("tipo_requerido", "Dime si es carro o moto.").WithField("tipo")
	ErrFloorPrice      = shared.Validation("precio_pendiente", "Hay ítems de la cotización sin precio: ponlos antes de confirmar.").WithField("cotizacion")
	ErrFloorExpense    = shared.Validation("gasto_invalido", "Cada gasto necesita qué fue y un valor mayor que cero.").WithField("gastos")
	ErrFloorItem       = shared.Validation("item_invalido", "Revisa la cantidad y el nombre de cada ítem de la cotización.").WithField("cotizacion")
	ErrFloorFinding    = shared.Validation("hallazgo_invalido", "Cada hallazgo necesita una descripción.").WithField("hallazgos")
	ErrFloorFuel       = shared.Validation("combustible_invalido", "Elige el nivel de combustible.").WithField("combustible")
	ErrFloorKm         = shared.Validation("kilometraje_invalido", "El kilometraje es un número sin puntos.").WithField("kilometraje")
	ErrFloorNeedsOrder = shared.Validation("orden_requerida", "Para eso primero hay que abrir la orden: dime la placa y qué trae.").WithField("placa")
)

// Validate revisa la propuesta antes de aplicarla. Las reglas de cada módulo (estados, ficha, hallazgos) las aplican
// sus propios casos de uso al confirmar.
func (p FloorProposal) Validate() error {
	if !p.Pending() {
		return ErrFloorEmpty
	}
	for _, check := range []func() error{p.validateOrder, p.validateReception, p.validateItems} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

// validateOrder: sin orden abierta, la propuesta tiene que abrir una (placa, y tipo y cliente si el vehículo es nuevo).
func (p FloorProposal) validateOrder() error {
	switch {
	case p.OrderID != "":
		return nil
	case !p.NewOrder:
		return ErrFloorNeedsOrder
	case p.Plate == nil || p.Plate.Value == "":
		return ErrFloorPlate
	case !p.VehicleNew:
		return nil
	case p.Kind != "carro" && p.Kind != "moto":
		return ErrFloorKind
	case p.Customer == nil || (p.Customer.ID == "" && (p.Customer.Phone == nil || p.Customer.Name == nil)):
		return ErrFloorCustomer
	}
	return nil
}

func (p FloorProposal) validateReception() error {
	if p.Fuel != nil && !slices.Contains(Fuels, p.Fuel.Value) {
		return ErrFloorFuel
	}
	if p.Km != nil {
		if n, err := strconv.Atoi(p.Km.Value); err != nil || n < 0 {
			return ErrFloorKm
		}
	}
	return nil
}

func (p FloorProposal) validateItems() error {
	for _, f := range p.Findings {
		if strings.TrimSpace(f.Description) == "" {
			return ErrFloorFinding
		}
	}
	for _, l := range p.Quote {
		if strings.TrimSpace(l.Name) == "" || l.Qty < 1 || l.Qty > 999 {
			return ErrFloorItem
		}
		if l.Price == nil || *l.Price <= 0 {
			return ErrFloorPrice
		}
	}
	for _, e := range p.Expenses {
		if strings.TrimSpace(e.Description) == "" || e.Value <= 0 {
			return ErrFloorExpense
		}
	}
	return nil
}

// HumanEdit aplica lo que la persona cambió en la ficha en vivo: lo que cambió queda con origen "pusiste" (ella lo
// escribió, así que un precio suyo vale). Lo que no cambió conserva su origen; no se pueden inventar orígenes.
func HumanEdit(prev, next FloorProposal) FloorProposal {
	out := keepIdentity(prev, next)
	out.Plate = editField(prev.Plate, out.Plate)
	out.Symptom = editField(prev.Symptom, out.Symptom)
	out.Km = editField(prev.Km, out.Km)
	out.Fuel = editField(prev.Fuel, out.Fuel)
	out.Move = editField(prev.Move, out.Move)
	out.Customer = editCustomer(prev.Customer, out.Customer)
	out.Sheet = editSheet(prev.Sheet, out.Sheet)
	for i := range out.Quote {
		out.Quote[i] = editLine(prev.Quote, out.Quote[i])
	}
	for i := range out.Expenses {
		out.Expenses[i] = editExpense(prev.Expenses, out.Expenses[i])
	}
	for i := range out.Findings {
		out.Findings[i] = editFinding(prev.Findings, out.Findings[i])
	}
	out.Photos = editPhotos(prev.Photos, out.Photos)
	for i, it := range out.Items {
		if old, ok := itemByKey(prev.Items, it.Key); ok && old.State == it.State && old.Note == it.Note {
			out.Items[i].Origin = old.Origin
		} else {
			out.Items[i].Origin = FromYou
		}
	}
	for i, n := range out.Notes {
		if !slices.ContainsFunc(prev.Notes, func(o Field) bool { return o.Value == n.Value }) {
			out.Notes[i].Origin = FromYou
		}
	}
	return out
}

// keepIdentity: lo que identifica la orden y el vehículo no se edita desde la ficha; sale del sistema.
func keepIdentity(prev, next FloorProposal) FloorProposal {
	out := next
	out.OrderID, out.OrderCode, out.OrderState, out.VehicleNew, out.Vehicle = prev.OrderID, prev.OrderCode, prev.OrderState, prev.VehicleNew, prev.Vehicle
	if prev.OrderID != "" {
		out.NewOrder = false
	}
	if prev.OrderID != "" || !prev.VehicleNew {
		out.Kind = prev.Kind
	}
	if !prev.VehicleNew || prev.OrderID != "" {
		out.Plate = prev.Plate
	}
	return out
}

func editCustomer(prev, next *FloorCustomer) *FloorCustomer {
	if next == nil {
		return nil
	}
	var pc FloorCustomer
	if prev != nil {
		pc = *prev
	}
	c := *next
	c.ID = pc.ID
	c.Phone, c.Name = editField(pc.Phone, c.Phone), editField(pc.Name, c.Name)
	return &c
}

func editSheet(prev, next map[string]Field) map[string]Field {
	if next == nil {
		return nil
	}
	sheet := map[string]Field{}
	for k, v := range next {
		if old, ok := prev[k]; ok && old.Value == v.Value {
			sheet[k] = old
		} else if strings.TrimSpace(v.Value) != "" {
			sheet[k] = Field{Value: strings.TrimSpace(v.Value), Origin: FromYou}
		}
	}
	return sheet
}

// editLine: un ítem nuevo es de la persona (sin servicio de la lista); un precio que cambió, también; sin precio no
// tiene origen. Lo demás conserva el suyo.
func editLine(prev []FloorLine, l FloorLine) FloorLine {
	old, ok := lineByID(prev, l.ID)
	switch {
	case !ok:
		l.ID, l.ServiceID, l.PriceOrigin = shared.NewID().String(), "", FromYou
	case !samePrice(old.Price, l.Price):
		l.ServiceID, l.PriceOrigin = old.ServiceID, FromYou
	default:
		l.ServiceID, l.PriceOrigin = old.ServiceID, old.PriceOrigin
	}
	if l.Price == nil {
		l.PriceOrigin = ""
	}
	return l
}

func editExpense(prev []FloorExpense, e FloorExpense) FloorExpense {
	old, ok := expenseByID(prev, e.ID)
	switch {
	case ok && old.Value == e.Value && old.Description == e.Description:
		e.Origin = old.Origin
	case !ok:
		e.ID, e.Origin = shared.NewID().String(), FromYou
	default:
		e.Origin = FromYou
	}
	return e
}

func editFinding(prev []FloorFinding, f FloorFinding) FloorFinding {
	old, ok := findingByID(prev, f.ID)
	switch {
	case ok && old == f:
		return f
	case !ok:
		f.ID = shared.NewID().String()
	}
	f.Origin = FromYou
	return f
}

// editPhotos: las fotos solo pueden ser las que mandó la persona; se quitan o se les cambia el lado, no se agregan.
func editPhotos(prev, next []FloorPhoto) []FloorPhoto {
	out := make([]FloorPhoto, 0, len(next))
	for _, ph := range next {
		if old, ok := photoByRef(prev, ph.Ref); ok {
			if old.Side != ph.Side {
				old.Side, old.Origin = ph.Side, FromYou
			}
			out = append(out, old)
		}
	}
	return out
}

func editField(prev, next *Field) *Field {
	if next == nil || strings.TrimSpace(next.Value) == "" {
		return nil
	}
	if prev != nil && prev.Value == next.Value {
		return prev
	}
	return &Field{Value: strings.TrimSpace(next.Value), Origin: FromYou}
}

func samePrice(a, b *int64) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

func lineByID(list []FloorLine, id string) (FloorLine, bool) {
	for _, l := range list {
		if l.ID == id && id != "" {
			return l, true
		}
	}
	return FloorLine{}, false
}

func expenseByID(list []FloorExpense, id string) (FloorExpense, bool) {
	for _, e := range list {
		if e.ID == id && id != "" {
			return e, true
		}
	}
	return FloorExpense{}, false
}

func findingByID(list []FloorFinding, id string) (FloorFinding, bool) {
	for _, f := range list {
		if f.ID == id && id != "" {
			return f, true
		}
	}
	return FloorFinding{}, false
}

func photoByRef(list []FloorPhoto, ref string) (FloorPhoto, bool) {
	for _, p := range list {
		if p.Ref == ref {
			return p, true
		}
	}
	return FloorPhoto{}, false
}

func itemByKey(list []FloorItem, key string) (FloorItem, bool) {
	for _, it := range list {
		if it.Key == key {
			return it, true
		}
	}
	return FloorItem{}, false
}

// SniffImage dice el tipo de una foto por su contenido (no por lo que diga el navegador): JPG, PNG o WEBP.
func SniffImage(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}
