package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/shared"
)

// Tools del asesor (ADR-0029). Cada una aplica sus reglas en código: el asesor puede pedir lo que quiera, pero solo
// ve datos del taller y del propio cliente, nunca el costo, y no puede duplicar órdenes ni tocar vehículos ajenos.

// Product es un producto de la vitrina de WhatsApp (repuesto o accesorio marcado por el taller). Sin costo.
type Product struct {
	ID          shared.ID
	Ref, Name   string
	Brand, Kind string
	Price       int64
	Stock       int
	HasPhoto    bool
}

func obj(props string, required ...string) json.RawMessage {
	req, _ := json.Marshal(required)
	if len(required) == 0 {
		req = []byte("[]")
	}
	return json.RawMessage(`{"type":"object","properties":{` + props + `},"required":` + string(req) + `,"additionalProperties":false}`)
}

// AdvisorTools es lo que el modelo puede usar.
var AdvisorTools = []ToolSpec{
	{Name: "servicios", Description: "Servicios del taller con su precio de mano de obra (null = el taller lo confirma). Úsala antes de dar cualquier precio de servicio y para ofrecer servicios al saludar.",
		Params: obj(`"tipo_vehiculo":{"type":"string","enum":["carro","moto",""],"description":"filtra por tipo; vacío = todos"}`)},
	{Name: "productos", Description: "Productos en vitrina (repuestos y accesorios que el taller vende por WhatsApp) con precio, si hay existencia y si tienen foto.",
		Params: obj(`"busqueda":{"type":"string","description":"palabra a buscar; vacío = todos"}`)},
	{Name: "enviar_foto", Description: "Envía al cliente la foto de un producto de la vitrina, junto a tu respuesta.",
		Params: obj(`"producto":{"type":"string","description":"la referencia del producto que devolvió productos"}`, "producto")},
	{Name: "horario", Description: "Horario del taller y si está abierto ahora.", Params: obj(``)},
	{Name: "mis_ordenes", Description: "Órdenes abiertas del cliente que escribe, con su estado. Solo las suyas.", Params: obj(``)},
	{Name: "agendar", Description: "Agenda la cita para recibir el vehículo (orden \"Por recibir\" con fecha y hora). Úsala apenas tengas la placa: te dice qué falta (día y hora, nombre), te devuelve el resumen para que el cliente confirme y, cuando el cliente dice que sí, agenda. Si ya tiene una orden por recibir, la cambia a la nueva fecha; si el vehículo ya está en el taller, devuelve su orden.",
		Params: obj(`"placa":{"type":"string","description":"placa tal como la escribió el cliente"},"sintoma":{"type":"string","description":"qué necesita o qué le pasa, en pocas palabras"},"fecha":{"type":"string","description":"día de la cita, AAAA-MM-DD (vacío si aún no lo sabes)"},"hora":{"type":"string","description":"hora de la cita, HH:MM en 24 horas (vacío si aún no la sabes)"},"nombre":{"type":"string","description":"nombre del cliente, solo si te lo dio y no lo tenías"}`, "placa", "sintoma")},
	{Name: "apartar_producto", Description: "El cliente quiere comprar un producto de la vitrina: avisa al taller para que una persona confirme el apartado y el pago.",
		Params: obj(`"producto":{"type":"string"},"cantidad":{"type":"integer","minimum":1,"maximum":20}`, "producto", "cantidad")},
	{Name: "pasar_a_persona", Description: "Pasa la conversación a una persona del taller (el cliente lo pide, está molesto o no lo puedes resolver).",
		Params: obj(`"motivo":{"type":"string"}`, "motivo")},
}

// turn es el estado de una respuesta del asesor: lo que consultó (para los guardias) y lo que debe enviar aparte.
type turn struct {
	taller   Taller
	tallerID shared.ID
	in       Inbound
	c        Customer
	quoted   map[int64]bool // precios que devolvió una tool: los únicos que el texto puede decir
	photos   []photo        // fotos a enviar después del texto
	handed   bool           // la conversación pasó a una persona
	products map[string]Product
	said     string    // lo que el cliente escribió en la conversación: agendar solo usa placas que estén ahí (HU-55)
	now      time.Time // para validar la fecha de la cita
}

func (t *turn) run(ctx context.Context, name, args string) string {
	var a struct {
		Tipo     string `json:"tipo_vehiculo"`
		Busqueda string `json:"busqueda"`
		Producto string `json:"producto"`
		Placa    string `json:"placa"`
		Sintoma  string `json:"sintoma"`
		Fecha    string `json:"fecha"`
		Hora     string `json:"hora"`
		Nombre   string `json:"nombre"`
		Cantidad int    `json:"cantidad"`
		Motivo   string `json:"motivo"`
	}
	if strings.TrimSpace(args) != "" && json.Unmarshal([]byte(args), &a) != nil {
		return result(map[string]any{"error": "argumentos inválidos"})
	}
	if name == "agendar" {
		out, err := t.book(ctx, a.Placa, a.Sintoma, a.Fecha, a.Hora, a.Nombre)
		if err != nil {
			return result(map[string]any{"error": "no se pudo agendar; di que una persona del taller confirma"})
		}
		return result(out)
	}
	out, err := t.exec(ctx, name, a.Tipo, a.Busqueda, a.Producto, a.Placa, a.Sintoma, a.Cantidad, a.Motivo)
	if err != nil {
		return result(map[string]any{"error": "no se pudo consultar; di que el taller confirma"})
	}
	return result(out)
}

func result(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (t *turn) exec(ctx context.Context, name, kind, q, product, plate, symptom string, qty int, reason string) (any, error) {
	switch name {
	case "servicios":
		services, _, err := t.taller.Catalog(ctx, t.tallerID)
		if err != nil {
			return nil, err
		}
		var list []map[string]any
		for _, sv := range services {
			if kind != "" && sv.Vehicle != "" && sv.Vehicle != "ambos" && sv.Vehicle != kind {
				continue
			}
			item := map[string]any{"nombre": sv.Name, "tipo_vehiculo": sv.Vehicle, "precio": nil}
			if sv.Price != nil {
				item["precio"] = *sv.Price
				t.quoted[*sv.Price] = true
			}
			list = append(list, item)
		}
		return map[string]any{"servicios": list}, nil
	case "productos":
		ps, err := t.taller.Showcase(ctx, t.tallerID, q)
		if err != nil {
			return nil, err
		}
		var list []map[string]any
		for _, p := range ps {
			t.products[p.Ref] = p
			t.quoted[p.Price] = true
			list = append(list, map[string]any{"referencia": p.Ref, "nombre": p.Name, "marca": p.Brand, "para": p.Kind, "precio": p.Price,
				"disponible": p.Stock > 0, "tiene_foto": p.HasPhoto})
		}
		return map[string]any{"productos": list}, nil
	case "enviar_foto":
		p, ok := t.product(ctx, product)
		switch {
		case !ok:
			return map[string]any{"enviada": false, "motivo": "no encontré ese producto en la vitrina; usa la referencia que devolvió productos"}, nil
		case !p.HasPhoto:
			return map[string]any{"enviada": false, "motivo": "ese producto no tiene foto"}, nil
		}
		if len(t.photos) < 3 {
			t.photos = append(t.photos, photo{ref: "repuesto:" + p.ID.String(), caption: p.Name + " · " + domain.Money(p.Price)})
		}
		return map[string]any{"enviada": true}, nil
	case "horario":
		prof, err := t.taller.Profile(ctx, t.tallerID)
		if err != nil {
			return nil, err
		}
		_, open, next, err := t.taller.Workshop(ctx, t.tallerID)
		return map[string]any{"horario": prof.Hours, "abierto_ahora": open, "proxima_apertura": next}, err
	case "mis_ordenes":
		orders, err := t.taller.OpenOrders(ctx, t.tallerID, t.c.ID)
		if err != nil {
			return nil, err
		}
		var list []map[string]any
		for _, o := range orders {
			item := map[string]any{"orden": o.Code, "placa": o.Plate, "vehiculo": o.Vehicle, "estado": stateText[o.State]}
			// HU-56: la última foto marcada para el cliente sale después de la respuesta (una por orden, máx. 3).
			if ref, err := t.taller.LastClientPhoto(ctx, t.tallerID, o.ID); err == nil && ref != "" && len(t.photos) < 3 && !t.hasPhoto(ref) {
				t.photos = append(t.photos, photo{ref: ref, caption: photoCaption(o)})
				item["foto"] = "se le envía la última foto del vehículo"
			}
			if o.PromisedAt != nil {
				item["prometida_para"] = o.PromisedAt.Format("2006-01-02 15:04")
			}
			list = append(list, item)
		}
		return map[string]any{"ordenes": list,
			"nota": "si el cliente reclama por la demora o está molesto, discúlpate y usa pasar_a_persona; no prometas llamadas ni tiempos"}, nil
	case "apartar_producto":
		p, ok := t.product(ctx, product)
		if !ok {
			return map[string]any{"apartado": false, "motivo": "ese producto no está en la vitrina"}, nil
		}
		t.handed = true
		if err := t.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true); err != nil {
			return nil, err
		}
		return map[string]any{"apartado": true, "producto": p.Name, "cantidad": max(qty, 1),
			"nota": "una persona del taller confirma el apartado y el pago por este chat"}, nil
	case "pasar_a_persona":
		t.handed = true
		return map[string]any{"ok": true, "nota": "dile que una persona del taller le escribe en cuanto pueda"},
			t.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true)
	}
	return map[string]any{"error": fmt.Sprintf("no existe la tool %q", name)}, nil
}

// book es la tool agendar (ADR-0029). Las reglas van en código y en este orden:
//  1. la placa la escribió el cliente (HU-55); si es de otro cliente lo revisa una persona; si ya tiene una orden que
//     no está por recibir, se devuelve esa;
//  2. día y hora: no en el pasado, hasta 30 días y dentro del horario de ese día;
//  3. el nombre, si el que tiene es de relleno;
//  4. la confirmación: la orden solo se crea (o se reagenda) si el último mensaje del cliente dice que sí. Si no, se
//     devuelve el resumen para que el asesor le pregunte. Así el modelo no puede agendar sin que el cliente confirme.
func (t *turn) book(ctx context.Context, plate, symptom, date, hour, name string) (any, error) {
	p := domain.FindPlate(plate)
	if p == "" {
		return map[string]any{"resultado": "placa_invalida", "nota": "pide la placa de nuevo, con letras y números"}, nil
	}
	if !strings.Contains(compact(t.said), p) { // HU-55: la placa la escribe el cliente, no la deduce la IA
		return map[string]any{"resultado": "placa_no_confirmada", "nota": "pídele al cliente que escriba la placa del vehículo que va a traer"}, nil
	}
	info, err := t.taller.PlateOf(ctx, t.tallerID, t.c.ID, p)
	if err != nil {
		return nil, err
	}
	switch {
	case info.Owner == "ajena": // de otro cliente: no se dice nada del vehículo y lo revisa una persona (HU-12)
		t.handed = true
		return map[string]any{"resultado": "revision", "nota": "di solo que una persona del taller le escribe para confirmar"},
			t.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true)
	case info.Open != nil && info.Open.State != "por_recibir": // el vehículo ya está en el taller
		return map[string]any{"resultado": "ya_tenia_orden", "orden": info.Code, "estado": stateText[info.Open.State]}, nil
	}
	if strings.TrimSpace(date) == "" || strings.TrimSpace(hour) == "" {
		return map[string]any{"resultado": "falta_fecha", "nota": "pregunta qué día y a qué hora lo trae"}, nil
	}
	when, err := domain.ParseWhen(date, hour, t.now)
	if err != nil {
		return map[string]any{"resultado": "fecha_invalida", "nota": "la cita debe ser desde ahora y hasta 30 días; pide otro día y hora"}, nil
	}
	open, hours, err := t.taller.OpenAt(ctx, t.tallerID, when)
	if err != nil {
		return nil, err
	}
	if !open {
		day := "ese día no abrimos"
		if hours != "" {
			day = "ese día atendemos de " + hours
		}
		return map[string]any{"resultado": "fuera_de_horario", "nota": day + "; ofrece otra hora o día dentro del horario"}, nil
	}
	if domain.IsPlaceholder(t.c.Name) && strings.TrimSpace(name) == "" {
		return map[string]any{"resultado": "falta_nombre", "nota": "pregúntale su nombre para la cita"}, nil
	}
	summary := map[string]any{"placa": p, "servicio": symptom, "cita": domain.FormatWhen(when)}
	if info.Open != nil {
		summary["orden"] = info.Code
	}
	if !domain.IsAffirmative(t.in.Text) {
		summary["resultado"] = "confirmar"
		summary["nota"] = "aún no está agendado: muéstrale este resumen y pregúntale si confirma; cuando diga que sí, vuelve a usar agendar"
		return summary, nil
	}
	if strings.TrimSpace(name) != "" {
		if _, err := t.taller.SetCustomerName(ctx, t.tallerID, t.c.ID, name); err != nil {
			return nil, err
		}
	}
	res, err := t.taller.CreateOrderFromPlate(ctx, t.tallerID, t.c.ID, p, symptom, &when)
	if err != nil {
		return nil, err
	}
	summary["orden"] = res.Code
	switch {
	case res.Owner == "ajena": // cambió entre la consulta y la creación
		t.handed = true
		return map[string]any{"resultado": "revision", "nota": "di solo que una persona del taller le escribe para confirmar"},
			t.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true)
	case res.Rescheduled:
		summary["resultado"] = "reagendado"
	case res.Open != nil:
		return map[string]any{"resultado": "ya_tenia_orden", "orden": res.Code, "estado": stateText[res.Open.State]}, nil
	default:
		summary["resultado"] = "agendado"
	}
	return summary, nil
}

// product busca un producto de la vitrina por referencia (o nombre), aunque el modelo no haya listado antes.
func (t *turn) product(ctx context.Context, ref string) (Product, bool) {
	if p, ok := t.products[ref]; ok {
		return p, true
	}
	ps, err := t.taller.Showcase(ctx, t.tallerID, "")
	if err != nil {
		return Product{}, false
	}
	want := domain.Normalize(ref)
	var hit *Product
	for i, p := range ps {
		t.products[p.Ref] = p
		name := domain.Normalize(p.Brand + " " + p.Name)
		if strings.EqualFold(p.Ref, ref) || domain.Normalize(p.Name) == want {
			hit = &ps[i]
			break
		}
		if want != "" && (strings.Contains(name, want) || strings.Contains(want, domain.Normalize(p.Name))) && hit == nil {
			hit = &ps[i]
		}
	}
	if hit == nil {
		return Product{}, false
	}
	t.quoted[hit.Price] = true
	return *hit, true
}

func (t *turn) hasPhoto(ref string) bool {
	for _, p := range t.photos {
		if p.ref == ref {
			return true
		}
	}
	return false
}

// compact deja el texto en mayúsculas sin espacios ni guiones, para buscar una placa escrita como "gno 667" o "GNO-667".
func compact(s string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "-", "", ".", "").Replace(s))
}

// photo es una foto de la vitrina con su pie (nombre y precio de la lista).
type photo struct{ ref, caption string }

// Tools ejecuta las tools del asesor para un cliente fuera del servicio (cmd/agent-eval con un taller de prueba).
type Tools struct{ t *turn }

// said es lo que el cliente escribió en la conversación (agendar solo usa placas que estén ahí); now, la hora actual.
func NewTools(tl Taller, tallerID shared.ID, c Customer, in Inbound, said string, now time.Time) *Tools {
	return &Tools{t: &turn{taller: tl, tallerID: tallerID, in: in, c: c, quoted: map[int64]bool{}, products: map[string]Product{},
		said: said, now: now}}
}

func (x *Tools) Run(ctx context.Context, name, args string) string { return x.t.run(ctx, name, args) }

// Catalog trae servicios y vitrina antes de llamar al modelo, para el prompt (ver turn.catalog).
func (x *Tools) Catalog(ctx context.Context) string { return x.t.catalog(ctx) }

// catalog consulta servicios y vitrina con las mismas tools y lo devuelve como texto para el prompt: así el asesor
// siempre sabe qué ofrece el taller y no tiene que adivinarlo, y esos precios quedan como consultados.
func (t *turn) catalog(ctx context.Context) string {
	return "Servicios del taller (consulta de hoy): " + t.run(ctx, "servicios", "{}") +
		"\nVitrina de productos (consulta de hoy): " + t.run(ctx, "productos", "{}")
}

// Quoted son los precios que devolvieron las tools: los únicos que el texto puede decir (domain.Guard).
func (x *Tools) Quoted() map[int64]bool { return x.t.quoted }

var stateText = map[string]string{"por_recibir": "por recibir", "recibido": "recibido", "en_diagnostico": "en diagnóstico",
	"esperando_aprobacion": "esperando tu aprobación de la cotización", "en_reparacion": "en reparación", "esperando_repuesto": "esperando un repuesto",
	"control_calidad": "en revisión final", "listo": "listo para recoger", "entregado": "entregado"}
