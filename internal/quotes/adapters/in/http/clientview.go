package http

import (
	"bytes"
	"net/http"
	"time"

	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/quotes/app"
)

// V-10 (ADR-0043, HU-109): lo que se le muestra al cliente en la pantalla del taller. Estos tipos son la lista
// blanca de lo que sale: lo que no está aquí (costos internos, gastos, notas, referencias) no puede salir.

type clientDetailDTO struct {
	Descripcion string `json:"descripcion"`
	Cantidad    int    `json:"cantidad"`
}

type clientLineDTO struct {
	Tipo         string            `json:"tipo"`
	Descripcion  string            `json:"descripcion"`
	Cantidad     int               `json:"cantidad"`
	Precio       int64             `json:"precioUnitario"`
	Subtotal     int64             `json:"subtotal"`
	DescuentoPct int               `json:"descuentoPct"`
	Descuento    int64             `json:"descuento"`
	Aprobada     *bool             `json:"aprobada"`
	Detalle      []clientDetailDTO `json:"detalle,omitempty"`
}

type clientQuoteDTO struct {
	Numero    int             `json:"numero"`
	Estado    string          `json:"estado"`
	Lineas    []clientLineDTO `json:"lineas"`
	Subtotal  int64           `json:"subtotal"`
	Descuento int64           `json:"descuento"`
	IvaPct    int             `json:"ivaPct"`
	Iva       int64           `json:"iva"`
	ConIva    bool            `json:"preciosConIva"`
	Total     int64           `json:"total"`
	EnviadaEn *time.Time      `json:"enviadaEn"`
	VenceEn   *time.Time      `json:"venceEn"`
	Vencida   bool            `json:"vencida"`
}

type clientDocDTO struct {
	Taller     string         `json:"taller"`
	Orden      string         `json:"orden"`
	Placa      string         `json:"placa"`
	Vehiculo   string         `json:"vehiculo"`
	Cliente    string         `json:"cliente"`
	Cotizacion clientQuoteDTO `json:"cotizacion"`
}

// authDTO es para quien atiende, no para el cliente: si ya autorizó sus datos y el texto que debe aceptar (HU-86).
type authDTO struct {
	ClienteID string `json:"clienteId"`
	Estado    string `json:"estado"`
	Texto     string `json:"texto"`
}

func toClientDoc(d app.ClientDoc) clientDocDTO {
	v := d.Quote
	q := clientQuoteDTO{Numero: v.Number, Estado: string(v.State), Lineas: make([]clientLineDTO, 0, len(v.Lines)), Subtotal: v.Totals.Gross,
		Descuento: v.Totals.Discount, IvaPct: v.VATPct, Iva: v.Totals.VAT, ConIva: v.PricesIncludeVAT, Total: v.Totals.Total,
		EnviadaEn: v.SentAt, VenceEn: v.ExpiresAt, Vencida: v.Expired}
	for _, l := range v.Lines {
		ld := clientLineDTO{Tipo: l.Kind, Descripcion: l.Description, Cantidad: l.Qty, Precio: l.UnitPrice, Subtotal: l.Subtotal,
			DescuentoPct: l.DiscountPct, Descuento: l.Discount, Aprobada: l.Approved}
		for _, x := range l.Detail {
			ld.Detalle = append(ld.Detalle, clientDetailDTO{Descripcion: x.Description, Cantidad: x.Qty})
		}
		q.Lineas = append(q.Lineas, ld)
	}
	return clientDocDTO{Taller: d.Workshop, Orden: d.OrderCode, Placa: d.Plate, Vehiculo: d.Vehicle, Cliente: d.CustomerName, Cotizacion: q}
}

// clientView devuelve la cotización vigente tal como la ve el cliente.
func (h *Handler) clientView(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	s, err := h.svc.ClientView(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{
		"documento":    toClientDoc(s.Doc),
		"autorizacion": authDTO{ClienteID: s.Auth.CustomerID.String(), Estado: s.Auth.State, Texto: h.consentText},
	}, nil
}

// pdf entrega la cotización en PDF carta. No se guarda en caché: lleva datos del cliente.
func (h *Handler) pdf(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	body, name, err := h.svc.PDF(r.Context(), p.TallerID, id)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	// ServeContent es el camino para entregar un archivo binario: respeta el tipo ya fijado y no lo interpreta como HTML.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
}
