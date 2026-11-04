package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/orders/app"
	"sweveh/internal/orders/domain"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
)

// FindingsHandler expone el diagnóstico en la orden (HU-103 propuesta). No muestra precios.
type FindingsHandler struct {
	svc   *app.Findings
	guard *authn.Guard
	log   *slog.Logger
}

func NewFindingsHandler(svc *app.Findings, guard *authn.Guard, log *slog.Logger) *FindingsHandler {
	return &FindingsHandler{svc: svc, guard: guard, log: log}
}

func (h *FindingsHandler) Routes(r chi.Router) {
	j := func(fn httpx.Func) http.HandlerFunc { return httpx.JSON(h.log, fn) }
	r.With(h.guard.Require(permRead)).Get("/orders/{id}/findings", j(h.list))
	r.With(h.guard.Require("order:diagnose")).Post("/orders/{id}/findings", j(h.add))
	r.With(h.guard.Require("order:diagnose")).Delete("/orders/{id}/findings/{fid}", j(h.remove))
}

type findingDTO struct {
	ID           string    `json:"id"`
	Sistema      string    `json:"sistema"`
	Descripcion  string    `json:"descripcion"`
	Gravedad     string    `json:"gravedad"`
	CodigosFalla []string  `json:"codigosFalla"`
	CreadoEn     time.Time `json:"creadoEn"`
	ManoObra     *int64    `json:"manoObra"`
}

func findingOut(f domain.Finding) findingDTO {
	codes := f.FaultCodes
	if codes == nil {
		codes = []string{}
	}
	return findingDTO{ID: f.ID.String(), Sistema: f.System, Descripcion: f.Description, Gravedad: f.Severity, CodigosFalla: codes, CreadoEn: f.CreatedAt, ManoObra: f.Labor}
}

func (h *FindingsHandler) list(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	list, err := h.svc.List(r.Context(), p.TallerID, id)
	out := make([]findingDTO, 0, len(list))
	for _, f := range list {
		out = append(out, findingOut(f))
	}
	return http.StatusOK, map[string]any{"hallazgos": out}, err
}

func (h *FindingsHandler) add(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Sistema      string   `json:"sistema"`
		Descripcion  string   `json:"descripcion"`
		Gravedad     string   `json:"gravedad"`
		CodigosFalla []string `json:"codigosFalla"`
		ManoObra     *int64   `json:"manoObra"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	f, err := h.svc.Add(r.Context(), app.FindingInput{TallerID: p.TallerID, OrderID: id, Actor: p.Actor(), System: q.Sistema,
		Description: q.Descripcion, Severity: q.Gravedad, FaultCodes: q.CodigosFalla, Labor: q.ManoObra})
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, findingOut(f), nil
}

func (h *FindingsHandler) remove(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	fid, err := httpx.PathID(r, "fid", domain.ErrFindingNotFound)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusNoContent, nil, h.svc.Remove(r.Context(), p.TallerID, id, fid, p.Actor())
}
