// Package http expone la cotización de una orden (HU-27 a HU-31).
package http

import (
	"log/slog"
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
	r.With(h.guard.Require("quote:send")).Post("/orders/{id}/quote/send", httpx.JSON(h.log, h.send))
	r.With(h.guard.Require("quote:register_response")).Post("/orders/{id}/quote/response", httpx.JSON(h.log, h.respond))
}

type lineDTO struct {
	ID          string  `json:"id"`
	Tipo        string  `json:"tipo"`
	RefID       *string `json:"refId"`
	Descripcion string  `json:"descripcion"`
	Cantidad    int     `json:"cantidad"`
	Precio      int64   `json:"precioUnitario"`
	Subtotal    int64   `json:"subtotal"`
	SinStock    bool    `json:"sinStock"`
	Aprobada    *bool   `json:"aprobada"`
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
}

func toDTO(q *domain.Quote) any {
	if q == nil {
		return map[string]any{"cotizacion": nil}
	}
	d := quoteDTO{ID: q.ID.String(), Numero: q.Number, Estado: string(q.State), Total: q.Total(), Lineas: []lineDTO{},
		EnviadaEn: q.SentAt, Respondida: q.RespondedAt, Canal: q.Channel}
	for _, l := range q.Lines {
		var ref *string
		if l.RefID != nil {
			s := l.RefID.String()
			ref = &s
		}
		d.Lineas = append(d.Lineas, lineDTO{ID: l.ID.String(), Tipo: l.Kind, RefID: ref, Descripcion: l.Description,
			Cantidad: l.Qty, Precio: l.UnitPrice, Subtotal: l.Subtotal(), SinStock: l.NoStock, Aprobada: l.Approved})
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
	Tipo        string  `json:"tipo"`
	RefID       *string `json:"refId"`
	Descripcion string  `json:"descripcion"`
	Cantidad    int     `json:"cantidad"`
	Precio      *int64  `json:"precioUnitario"`
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
	in := app.LineInput{Kind: q.Tipo, Description: q.Descripcion, Qty: q.Cantidad, UnitPrice: q.Precio}
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
