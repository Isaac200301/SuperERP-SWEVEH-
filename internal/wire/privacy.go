package wire

import (
	"context"
	"net/http"
	"time"

	customersdomain "sweveh/internal/customers/domain"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// customerEraser borra lo que identifica al titular en los otros módulos (HU-87), cada cosa por su fachada y en la
// transacción del borrado: placas (vehículos y su copia en las órdenes), quién recogió, borradores y conversaciones.
type customerEraser struct{ s *Services }

func (c customerEraser) AnonymizeCustomer(ctx context.Context, tallerID shared.ID, cu customersdomain.Customer) (map[string]int, error) {
	plates, err := c.s.M.VehDir.AnonymizeCustomer(ctx, tallerID, cu.ID)
	if err != nil {
		return nil, err
	}
	out := map[string]int{"vehiculos": len(plates)}
	if out["ordenes"], err = c.s.M.OrdersF.AnonymizeCustomer(ctx, tallerID, cu.ID, plates); err != nil {
		return nil, err
	}
	if out["borradores"], err = c.s.Assistant.AnonymizePhone(ctx, tallerID, cu.Phone); err != nil {
		return nil, err
	}
	if out["mensajes"], err = c.s.Messaging.AnonymizeCustomer(ctx, tallerID, cu.ID); err != nil {
		return nil, err
	}
	return out, nil
}

// exportCustomer arma la copia de los datos del titular (HU-87): lo que el taller guarda de él, sus autorizaciones,
// vehículos, órdenes con su total aprobado y los mensajes de WhatsApp. La web la descarga como archivo JSON.
func (h *handler) exportCustomer(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", customersdomain.ErrCustomerNotFound)
	if err != nil {
		return 0, nil, err
	}
	ctx := r.Context()
	pd, err := h.s.M.Customers.PersonalData(ctx, p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	c := pd.Customer
	consents := make([]map[string]any, 0, len(pd.Consents))
	for _, x := range pd.Consents {
		consents = append(consents, map[string]any{"estado": x.State, "texto": x.Text, "version": x.Version, "canal": x.Channel, "fecha": x.At})
	}
	vs, err := h.s.M.VehDir.ByCustomer(ctx, p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	vehicles := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		vehicles = append(vehicles, map[string]any{"placa": v.Plate, "tipo": v.Kind, "marca": v.Brand, "linea": v.Line, "modelo": v.Year})
	}
	os, err := h.s.M.OrdersF.ByCustomer(ctx, p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	ids := make([]shared.ID, 0, len(os))
	for _, o := range os {
		ids = append(ids, o.ID)
	}
	var totals map[shared.ID]int64
	if err := h.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		var err error
		totals, err = h.s.M.QuotesF.ApprovedTotals(ctx, p.TallerID, ids)
		return err
	}); err != nil {
		return 0, nil, err
	}
	orders := make([]map[string]any, 0, len(os))
	for _, o := range os {
		item := map[string]any{"orden": o.Code, "placa": o.Plate, "estado": o.State, "creada": o.CreatedAt, "cerrada": o.ClosedAt}
		if t, ok := totals[o.ID]; ok {
			item["totalAprobado"] = t
		}
		orders = append(orders, item)
	}
	ms, err := h.s.Messaging.MessagesOf(ctx, p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	messages := make([]map[string]any, 0, len(ms))
	for _, m := range ms {
		messages = append(messages, map[string]any{"fecha": m.CreatedAt, "direccion": m.Direction, "texto": m.Body})
	}
	return http.StatusOK, map[string]any{
		"generadoEn": time.Now().UTC(),
		"cliente": map[string]any{"nombre": c.Name, "celular": shared.NationalNumber(c.Phone), "correo": c.Email,
			"autorizacion": string(c.Consent), "registradoEn": c.CreatedAt},
		"autorizaciones": consents, "vehiculos": vehicles, "ordenes": orders, "mensajes": messages,
	}, nil
}
