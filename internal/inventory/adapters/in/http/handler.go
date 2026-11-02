// Package http expone el inventario (HU-32 a HU-38). El costo solo sale con part:cost (RF-21, HU-81).
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/inventory/app"
	"sweveh/internal/inventory/domain"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

type Handler struct {
	svc   *app.Service
	guard *authn.Guard
	log   *slog.Logger
}

func NewHandler(svc *app.Service, g *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{svc: svc, guard: g, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require("part:read")).Get("/parts", httpx.JSON(h.log, h.search))
	r.With(h.guard.Require("part:read")).Get("/parts/low", httpx.JSON(h.log, h.low))
	r.With(h.guard.Require("part:manage")).Post("/parts", httpx.JSON(h.log, h.create))
	r.With(h.guard.Require("part:manage")).Patch("/parts/{id}", httpx.JSON(h.log, h.update))
	r.With(h.guard.Require("part:manage")).Post("/parts/{id}/entries", httpx.JSON(h.log, h.entry))
	r.With(h.guard.Require("part:return")).Post("/parts/{id}/returns", httpx.JSON(h.log, h.ret))
}

type dto struct {
	ID         string `json:"id"`
	Referencia string `json:"referencia"`
	Nombre     string `json:"nombre"`
	Tipo       string `json:"tipoVehiculo"`
	Costo      *int64 `json:"costo,omitempty"`
	Precio     int64  `json:"precioVenta"`
	Existencia int    `json:"existencia"`
	Minimo     int    `json:"minimo"`
	BajoMinimo bool   `json:"bajoMinimo"`
	Activo     bool   `json:"activo"`
	Version    int    `json:"version"`
}

func (h *Handler) toDTO(p domain.Part, pr reqctx.Principal) dto {
	d := dto{ID: p.ID.String(), Referencia: p.Ref, Nombre: p.Name, Tipo: p.Kind, Precio: p.Price, Existencia: p.Stock,
		Minimo: p.Min, BajoMinimo: p.LowStock(), Activo: p.Active, Version: p.Version}
	if h.guard.Can(pr, "part:cost") {
		c := p.Cost
		d.Costo = &c
	}
	return d
}

func (h *Handler) many(list []domain.Part, p reqctx.Principal) map[string]any {
	out := make([]dto, 0, len(list))
	for _, x := range list {
		out = append(out, h.toDTO(x, p))
	}
	return map[string]any{"repuestos": out}
}

func (h *Handler) search(r *http.Request, p reqctx.Principal) (int, any, error) {
	all := r.URL.Query().Get("todos") == "1" && h.guard.Can(p, "part:manage")
	list, err := h.svc.Search(r.Context(), p.TallerID, r.URL.Query().Get("q"), all)
	return http.StatusOK, h.many(list, p), err
}

func (h *Handler) low(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.svc.Low(r.Context(), p.TallerID)
	return http.StatusOK, h.many(list, p), err
}

type createReq struct {
	Referencia string `json:"referencia"`
	Nombre     string `json:"nombre"`
	Tipo       string `json:"tipoVehiculo"`
	Costo      int64  `json:"costo"`
	Precio     int64  `json:"precioVenta"`
	Existencia int    `json:"existencia"`
	Minimo     int    `json:"minimo"`
}

func (h *Handler) create(r *http.Request, p reqctx.Principal) (int, any, error) {
	var q createReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	part, err := h.svc.Create(r.Context(), p.Actor(), domain.Part{TallerID: p.TallerID, Ref: q.Referencia, Name: q.Nombre,
		Kind: q.Tipo, Cost: q.Costo, Price: q.Precio, Stock: q.Existencia, Min: q.Minimo})
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, h.toDTO(part, p), nil
}

type patchReq struct {
	Nombre  *string `json:"nombre"`
	Tipo    *string `json:"tipoVehiculo"`
	Costo   *int64  `json:"costo"`
	Precio  *int64  `json:"precioVenta"`
	Minimo  *int    `json:"minimo"`
	Activo  *bool   `json:"activo"`
	Version int     `json:"version"`
}

func (h *Handler) update(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q patchReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	part, err := h.svc.Update(r.Context(), p.Actor(), p.TallerID, id, q.Version, app.Patch{Name: q.Nombre, Kind: q.Tipo,
		Cost: q.Costo, Price: q.Precio, Min: q.Minimo, Active: q.Activo})
	return http.StatusOK, h.toDTO(part, p), err
}

type qtyReq struct {
	Cantidad int    `json:"cantidad"`
	OrdenID  string `json:"ordenId"`
}

func (h *Handler) entry(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q qtyReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	part, err := h.svc.AddEntry(r.Context(), p.Actor(), p.TallerID, id, q.Cantidad)
	return http.StatusOK, h.toDTO(part, p), err
}

func (h *Handler) ret(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q qtyReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	oid, err := shared.ParseID(q.OrdenID)
	if err != nil {
		return 0, nil, shared.Validation("orden_requerida", "Indica la orden de la que vuelve el repuesto.").WithField("ordenId")
	}
	return http.StatusNoContent, nil, h.svc.Return(r.Context(), p.Actor(), p.TallerID, id, oid, q.Cantidad)
}
