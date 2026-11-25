// Package http expone la cotización de una orden (HU-27 a HU-31).
package http

import (
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/quotes/app"
	"sweveh/internal/quotes/domain"
	"sweveh/internal/shared"
)

// errVersionNeeded: las ediciones nuevas de la cotización piden la versión que tenía la pantalla (R-07).
var errVersionNeeded = shared.Validation("version_requerida", "Falta la versión de la cotización; recarga la pantalla.").WithField("version")

var errOrder = shared.NotFound("orden_no_encontrada", "No encontramos esa orden en tu taller.")

type Handler struct {
	svc   *app.Quotes
	guard *authn.Guard
	log   *slog.Logger
}

func NewHandler(s *app.Quotes, g *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{svc: s, guard: g, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require("quote:read")).Get("/orders/{id}/quote", httpx.JSON(h.log, h.get))
	r.With(h.guard.Require("quote:edit")).Post("/orders/{id}/quote/lines", httpx.JSON(h.log, h.add))
	r.With(h.guard.Require("quote:edit")).Delete("/orders/{id}/quote/lines/{lineId}", httpx.JSON(h.log, h.remove))
	r.With(h.guard.Require("quote:edit")).Patch("/orders/{id}/quote/lines/{lineId}", httpx.JSON(h.log, h.update))
	// V-07 (ADR-0040): detalle interno de una partida y "Mostrar detalle al cliente".
	r.With(h.guard.Require("quote:edit")).Put("/orders/{id}/quote/lines/{lineId}/detail", httpx.JSON(h.log, h.setDetail))
	r.With(h.guard.Require("quote:edit")).Patch("/orders/{id}/quote", httpx.JSON(h.log, h.setQuote))
	r.With(h.guard.Require("quote:send")).Post("/orders/{id}/quote/send", httpx.JSON(h.log, h.send))
	r.With(h.guard.Require("quote:register_response")).Post("/orders/{id}/quote/response", httpx.JSON(h.log, h.respond))
	r.With(h.guard.Require("quote:send")).Post("/orders/{id}/quote/renew", httpx.JSON(h.log, h.renew))
}

// hoursCent pasa horas con decimales (1,5) a centésimas (150).
func hoursCent(h *float64) *int {
	if h == nil {
		return nil
	}
	c := int(math.Round(*h * 100))
	return &c
}

func hoursOut(c *int) *float64 {
	if c == nil {
		return nil
	}
	h := float64(*c) / 100
	return &h
}

type lineDTO struct {
	ID          string   `json:"id"`
	Tipo        string   `json:"tipo"`
	RefID       *string  `json:"refId"`
	Descripcion string   `json:"descripcion"`
	Cantidad    int      `json:"cantidad"`
	Precio      int64    `json:"precioUnitario"`
	Subtotal    int64    `json:"subtotal"`
	SinStock    bool     `json:"sinStock"`
	PrecioLibre bool     `json:"precioLibre"`
	Aprobada    *bool    `json:"aprobada"`
	Descuento   int      `json:"descuentoPct"`
	Horas       *float64 `json:"horas"`
	// Detalle: solo en partidas; interno del taller, no suma al total (V-07).
	Detalle *[]detailDTO `json:"detalle,omitempty"`
}

type detailDTO struct {
	Descripcion string `json:"descripcion"`
	Cantidad    int    `json:"cantidad"`
	ValorRef    *int64 `json:"valorRef"`
}

type quoteDTO struct {
	ID         string     `json:"id"`
	Numero     int        `json:"numero"`
	Estado     string     `json:"estado"`
	Total      int64      `json:"total"`
	Lineas     []lineDTO  `json:"lineas"`
	EnviadaEn  *time.Time `json:"enviadaEn"`
	Respondida *time.Time `json:"respondidaEn"`
	Canal      *string    `json:"canalRespuesta"`
	Subtotal   int64      `json:"subtotal"`
	Descuento  int64      `json:"descuento"`
	IvaPct     int        `json:"ivaPct"`
	Iva        int64      `json:"iva"`
	ConIva     bool       `json:"preciosConIva"`
	VenceEn    *time.Time `json:"venceEn"`
	Vencida    bool       `json:"vencida"`
	Mostrar    bool       `json:"mostrarDetalle"`
	Version    int        `json:"version"` // If-Match de las ediciones (R-07)
}

func toDTO(q *domain.Quote) any {
	if q == nil {
		return map[string]any{"cotizacion": nil}
	}
	t := q.Totals()
	d := quoteDTO{ID: q.ID.String(), Numero: q.Number, Estado: string(q.State), Total: t.Total, Lineas: []lineDTO{},
		EnviadaEn: q.SentAt, Respondida: q.RespondedAt, Canal: q.Channel, Subtotal: t.Gross, Descuento: t.Discount, IvaPct: q.VATPct,
		Iva: t.VAT, ConIva: q.PricesIncludeVAT, VenceEn: q.ExpiresAt, Mostrar: q.ShowDetail, Version: q.Version,
		Vencida: (q.State == domain.Sent || q.State == domain.Doubts) && q.Expired(time.Now())}
	for _, l := range q.Lines {
		var ref *string
		if l.RefID != nil {
			s := l.RefID.String()
			ref = &s
		}
		ld := lineDTO{ID: l.ID.String(), Tipo: l.Kind, RefID: ref, Descripcion: l.Description,
			Cantidad: l.Qty, Precio: l.UnitPrice, Subtotal: l.Subtotal(), SinStock: l.NoStock, Aprobada: l.Approved, PrecioLibre: l.FreePrice,
			Descuento: l.DiscountPct, Horas: hoursOut(l.HoursCent)}
		if l.Kind == domain.KindPartida { // las partidas siempre traen su detalle, aunque esté vacío
			det := []detailDTO{}
			for _, it := range l.Detail {
				det = append(det, detailDTO{Descripcion: it.Description, Cantidad: it.Qty, ValorRef: it.ValueRef})
			}
			ld.Detalle = &det
		}
		d.Lineas = append(d.Lineas, ld)
	}
	return map[string]any{"cotizacion": d}
}

func (h *Handler) get(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	q, err := h.svc.ForOrder(r.Context(), p.TallerID, id)
	return http.StatusOK, toDTO(q), err
}

type addReq struct {
	Tipo        string   `json:"tipo"`
	RefID       *string  `json:"refId"`
	Descripcion string   `json:"descripcion"`
	Cantidad    int      `json:"cantidad"`
	Precio      *int64   `json:"precioUnitario"`
	Horas       *float64 `json:"horas"` // mano de obra por horas × tarifa del taller
}

func (h *Handler) add(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	var q addReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	in := app.LineInput{Kind: q.Tipo, Description: q.Descripcion, Qty: q.Cantidad, UnitPrice: q.Precio, HoursCent: hoursCent(q.Horas),
		SaveToList: h.guard.Can(p, "service:manage")}
	if in.HoursCent != nil && in.Qty == 0 {
		in.Qty = 1
	}
	if q.RefID != nil {
		ref, err := shared.ParseID(*q.RefID)
		if err != nil {
			return 0, nil, domain.ErrLine
		}
		in.RefID = &ref
	}
	out, err := h.svc.AddLine(r.Context(), p.TallerID, id, p.Actor(), in)
	return http.StatusOK, toDTO(&out), err
}

func (h *Handler) update(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	lid, err := httpx.PathID(r, "lineId", domain.ErrLineNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Cantidad  *int     `json:"cantidad"`
		Precio    *int64   `json:"precioUnitario"`
		Horas     *float64 `json:"horas"`
		Descuento *int     `json:"descuentoPct"`
		Nombre    *string  `json:"descripcion"` // partida o ítem libre (V-07)
		Version   *int     `json:"version"`     // la de la pantalla; obligatoria para renombrar
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.Descuento != nil && !h.guard.Can(p, "quote:discount") {
		return 0, nil, domain.ErrDiscountDenied
	}
	out, err := h.svc.UpdateLine(r.Context(), p.TallerID, id, lid, p.Actor(), app.LineChange{Qty: q.Cantidad, Price: q.Precio,
		HoursCent: hoursCent(q.Horas), DiscountPct: q.Descuento, Name: q.Nombre, Version: q.Version})
	return http.StatusOK, toDTO(&out), err
}

// setDetail reemplaza el detalle interno de una partida (V-07).
func (h *Handler) setDetail(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	lid, err := httpx.PathID(r, "lineId", domain.ErrLineNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Detalle []detailDTO `json:"detalle"`
		Version *int        `json:"version"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	items := make([]domain.DetailItem, 0, len(q.Detalle))
	for _, d := range q.Detalle {
		items = append(items, domain.DetailItem{Description: d.Descripcion, Qty: d.Cantidad, ValueRef: d.ValorRef})
	}
	if q.Version == nil {
		return 0, nil, errVersionNeeded
	}
	out, err := h.svc.SetDetail(r.Context(), p.TallerID, id, lid, p.Actor(), *q.Version, items)
	return http.StatusOK, toDTO(&out), err
}

// setQuote cambia lo de la cotización entera: hoy, "Mostrar detalle al cliente" (V-07).
func (h *Handler) setQuote(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Mostrar *bool `json:"mostrarDetalle"`
		Version *int  `json:"version"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.Mostrar == nil {
		return 0, nil, shared.Validation("mostrar_detalle_requerido", "Indica si el cliente ve el detalle.").WithField("mostrarDetalle")
	}
	if q.Version == nil {
		return 0, nil, errVersionNeeded
	}
	out, err := h.svc.SetShowDetail(r.Context(), p.TallerID, id, p.Actor(), *q.Version, *q.Mostrar)
	return http.StatusOK, toDTO(&out), err
}

func (h *Handler) renew(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	out, err := h.svc.Renew(r.Context(), p.TallerID, id, p.Actor())
	return http.StatusOK, toDTO(&out), err
}

func (h *Handler) remove(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	lid, err := httpx.PathID(r, "lineId", domain.ErrLineNotFound)
	if err != nil {
		return 0, nil, err
	}
	out, err := h.svc.RemoveLine(r.Context(), p.TallerID, id, lid, p.Actor())
	return http.StatusOK, toDTO(&out), err
}

func (h *Handler) send(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	out, err := h.svc.Send(r.Context(), p.TallerID, id, p.Actor())
	return http.StatusOK, toDTO(&out), err
}

type respondReq struct {
	Decision string   `json:"decision"`
	Canal    string   `json:"canal"`
	Items    []string `json:"itemsAprobados"`
}

// respond registra una respuesta tomada por teléfono o en mostrador (HU-29); el botón de WhatsApp entra por el webhook.
func (h *Handler) respond(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errOrder)
	if err != nil {
		return 0, nil, err
	}
	var q respondReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.Canal != "telefono" && q.Canal != "mostrador" {
		return 0, nil, shared.Validation("canal_invalido", "Indica si la respuesta llegó por teléfono o en el mostrador.").WithField("canal")
	}
	items := map[shared.ID]bool{}
	for _, s := range q.Items {
		if lid, err := shared.ParseID(s); err == nil {
			items[lid] = true
		}
	}
	out, err := h.svc.Respond(r.Context(), app.RespondCmd{TallerID: p.TallerID, OrderID: id, Actor: p.Actor(),
		Response: domain.Response{Decision: q.Decision, Items: items, Channel: q.Canal}})
	return http.StatusOK, toDTO(&out), err
}
