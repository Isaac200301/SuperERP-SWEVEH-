package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	{Name: "agendar", Description: "Registra el vehículo del cliente para recibirlo (orden \"Por recibir\"). Si ya tiene una orden abierta, la devuelve en lugar de crear otra.",
		Params: obj(`"placa":{"type":"string","description":"placa tal como la escribió el cliente"},"sintoma":{"type":"string","description":"qué necesita o qué le pasa, en pocas palabras"}`, "placa", "sintoma")},
	{Name: "apartar_producto", Description: "El cliente quiere comprar un producto de la vitrina: avisa al taller para que una persona confirme el apartado y el pago.",
		Params: obj(`"producto":{"type":"string"},"cantidad":{"type":"integer","minimum":1,"maximum":20}`, "producto", "cantidad")},
	{Name: "pasar_a_persona", Description: "Pasa la conversación a una persona del taller (el cliente lo pide, está molesto o no lo puedes resolver).",
		Params: obj(`"motivo":{"type":"string"}`, "motivo")},
}

// turn es el estado de una respuesta del asesor: lo que consultó (para los guardias) y lo que debe enviar aparte.
type turn struct {
	s        *Service
	tallerID shared.ID
	in       Inbound
	c        Customer
	quoted   map[int64]bool // precios que devolvió una tool: los únicos que el texto puede decir
	photos   []photo        // fotos a enviar después del texto
	handed   bool           // la conversación pasó a una persona
	products map[string]Product
}

func (t *turn) run(ctx context.Context, name, args string) string {
	var a struct {
		Tipo     string `json:"tipo_vehiculo"`
		Busqueda string `json:"busqueda"`
		Producto string `json:"producto"`
		Placa    string `json:"placa"`
		Sintoma  string `json:"sintoma"`
		Cantidad int    `json:"cantidad"`
		Motivo   string `json:"motivo"`
	}
	if strings.TrimSpace(args) != "" && json.Unmarshal([]byte(args), &a) != nil {
		return result(map[string]any{"error": "argumentos inválidos"})
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
		services, _, err := t.s.taller.Catalog(ctx, t.tallerID)
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
		ps, err := t.s.taller.Showcase(ctx, t.tallerID, q)
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
		if !ok || !p.HasPhoto {
			return map[string]any{"enviada": false, "motivo": "ese producto no tiene foto"}, nil
		}
		if len(t.photos) < 3 {
			t.photos = append(t.photos, photo{ref: "repuesto:" + p.ID.String(), caption: p.Name + " · " + domain.Money(p.Price)})
		}
		return map[string]any{"enviada": true}, nil
	case "horario":
		prof, err := t.s.taller.Profile(ctx, t.tallerID)
		if err != nil {
			return nil, err
		}
		_, open, next, err := t.s.taller.Workshop(ctx, t.tallerID)
		return map[string]any{"horario": prof.Hours, "abierto_ahora": open, "proxima_apertura": next}, err
	case "mis_ordenes":
		orders, err := t.s.taller.OpenOrders(ctx, t.tallerID, t.c.ID)
		if err != nil {
			return nil, err
		}
		var list []map[string]any
		for _, o := range orders {
			item := map[string]any{"orden": o.Code, "placa": o.Plate, "vehiculo": o.Vehicle, "estado": stateText[o.State]}
			if o.PromisedAt != nil {
				item["prometida_para"] = o.PromisedAt.Format("2006-01-02 15:04")
			}
			list = append(list, item)
		}
		return map[string]any{"ordenes": list}, nil
	case "agendar":
		p := domain.FindPlate(plate)
		if p == "" {
			return map[string]any{"resultado": "placa_invalida", "nota": "pide la placa de nuevo, con letras y números"}, nil
		}
		info, err := t.s.taller.CreateOrderFromPlate(ctx, t.tallerID, t.c.ID, p, symptom)
		if err != nil {
			return nil, err
		}
		switch {
		case info.Owner == "ajena": // de otro cliente: no se dice nada del vehículo y lo revisa una persona (HU-12)
			t.handed = true
			return map[string]any{"resultado": "revision", "nota": "una persona del taller revisa esa placa y le escribe; no des más detalles"},
				t.s.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true)
		case info.Open != nil:
			return map[string]any{"resultado": "ya_tenia_orden", "orden": info.Code, "estado": stateText[info.Open.State]}, nil
		}
		return map[string]any{"resultado": "agendado", "orden": info.Code, "placa": p}, nil
	case "apartar_producto":
		p, ok := t.product(ctx, product)
		if !ok {
			return map[string]any{"apartado": false, "motivo": "ese producto no está en la vitrina"}, nil
		}
		t.handed = true
		if err := t.s.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true); err != nil {
			return nil, err
		}
		return map[string]any{"apartado": true, "producto": p.Name, "cantidad": max(qty, 1),
			"nota": "una persona del taller confirma el apartado y el pago por este chat"}, nil
	case "pasar_a_persona":
		t.handed = true
		return map[string]any{"ok": true, "nota": "dile que una persona del taller le escribe en cuanto pueda"},
			t.s.taller.SetMode(ctx, t.tallerID, t.in.ConversationID, "humano", true)
	}
	return map[string]any{"error": fmt.Sprintf("no existe la tool %q", name)}, nil
}

// product busca un producto de la vitrina por referencia (o nombre), aunque el modelo no haya listado antes.
func (t *turn) product(ctx context.Context, ref string) (Product, bool) {
	if p, ok := t.products[ref]; ok {
		return p, true
	}
	ps, err := t.s.taller.Showcase(ctx, t.tallerID, "")
	if err != nil {
		return Product{}, false
	}
	for _, p := range ps {
		t.products[p.Ref] = p
		if strings.EqualFold(p.Ref, ref) || strings.EqualFold(p.Name, ref) {
			t.quoted[p.Price] = true
			return p, true
		}
	}
	return Product{}, false
}

// photo es una foto de la vitrina con su pie (nombre y precio de la lista).
type photo struct{ ref, caption string }

var stateText = map[string]string{"por_recibir": "por recibir", "recibido": "recibido", "en_diagnostico": "en diagnóstico",
	"esperando_aprobacion": "esperando tu aprobación de la cotización", "en_reparacion": "en reparación", "esperando_repuesto": "esperando un repuesto",
	"control_calidad": "en revisión final", "listo": "listo para recoger", "entregado": "entregado"}
