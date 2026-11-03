// Package http expone el catálogo de servicios (HU-04, HU-07).
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/catalog/app"
	"sweveh/internal/catalog/domain"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
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
	r.With(h.guard.Require("service:read")).Get("/services", httpx.JSON(h.log, h.list))
	r.With(h.guard.Require("service:manage")).Post("/services", httpx.JSON(h.log, h.create))
	r.With(h.guard.Require("service:manage")).Patch("/services/{id}", httpx.JSON(h.log, h.update))
	r.With(h.guard.Require("service:manage")).Delete("/services/{id}", httpx.JSON(h.log, h.remove))
}

type dto struct {
	ID          string  `json:"id"`
	Nombre      string  `json:"nombre"`
	Tipo        string  `json:"tipoVehiculo"`
	Precio      *int64  `json:"precio"`
	DuracionMin *int    `json:"duracionMin"`
	Activo      bool    `json:"activo"`
	Version     int     `json:"version"`
	Categoria   *string `json:"categoria"`
	Garantia    *int    `json:"garantiaDias"`
}

func toDTO(s domain.Service) dto {
	return dto{ID: s.ID.String(), Nombre: s.Name, Tipo: s.Kind, Precio: s.Price, DuracionMin: s.DurationMin, Activo: s.Active, Version: s.Version,
		Categoria: s.Category, Garantia: s.WarrantyDay}
}

func (h *Handler) list(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.svc.List(r.Context(), p.TallerID, r.URL.Query().Get("todos") == "1")
	out := make([]dto, 0, len(list))
	for _, s := range list {
		out = append(out, toDTO(s))
	}
	return http.StatusOK, map[string]any{"servicios": out}, err
}

type createReq struct {
	Nombre      string  `json:"nombre"`
	Tipo        string  `json:"tipoVehiculo"`
	Precio      *int64  `json:"precio"`
	DuracionMin *int    `json:"duracionMin"`
	Categoria   *string `json:"categoria"`
	Garantia    *int    `json:"garantiaDias"`
}

func (h *Handler) create(r *http.Request, p reqctx.Principal) (int, any, error) {
	var req createReq
	if err := httpx.Decode(r, &req); err != nil {
		return 0, nil, err
	}
	s, err := h.svc.Create(r.Context(), p.Actor(), domain.Service{TallerID: p.TallerID, Name: req.Nombre, Kind: req.Tipo,
		Price: req.Precio, DurationMin: req.DuracionMin, Category: req.Categoria, WarrantyDay: req.Garantia})
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, toDTO(s), nil
}

// patchReq usa punteros dobles para distinguir "no vino" de "vino en null" (quitar el precio).
type patchReq struct {
	Nombre      *string                `json:"nombre"`
	Tipo        *string                `json:"tipoVehiculo"`
	Precio      httpx.Optional[int64]  `json:"precio"`
	DuracionMin httpx.Optional[int]    `json:"duracionMin"`
	Activo      *bool                  `json:"activo"`
	Version     int                    `json:"version"`
	Categoria   httpx.Optional[string] `json:"categoria"`
	Garantia    httpx.Optional[int]    `json:"garantiaDias"`
}

func (h *Handler) update(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrNotFound)
	if err != nil {
		return 0, nil, err
	}
	var req patchReq
	if err := httpx.Decode(r, &req); err != nil {
		return 0, nil, err
	}
	s, err := h.svc.Update(r.Context(), p.Actor(), p.TallerID, id, req.Version, app.Patch{
		Name: req.Nombre, Kind: req.Tipo, Price: req.Precio.Ptr(), DurationMin: req.DuracionMin.Ptr(), Active: req.Activo,
		Category: req.Categoria.Ptr(), WarrantyDay: req.Garantia.Ptr(),
	})
	return http.StatusOK, toDTO(s), err
}

func (h *Handler) remove(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrNotFound)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, h.svc.Remove(r.Context(), p.Actor(), p.TallerID, id)
}
