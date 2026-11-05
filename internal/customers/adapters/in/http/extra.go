package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/customers/app"
	"sweveh/internal/customers/domain"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Texto y versión de la autorización del cliente final. Borrador hasta la revisión jurídica (AQ-10).
const (
	ConsentVersion = "autorizacion-cliente-borrador-2026-11-01"
	ConsentText    = "Autorizo al taller a tratar mis datos (nombre, celular y datos del vehículo) para atender mi vehículo y avisarme por WhatsApp."
)

// ExtraRoutes: lista de clientes y autorización en mostrador (HU-86).
func (h *Handler) ExtraRoutes(r chi.Router) {
	r.With(h.guard.Require(permRead)).Get("/customers", httpx.JSON(h.log, h.list))
	r.With(h.guard.Require(permCreate)).Post("/customers/{id}/consent", httpx.JSON(h.log, h.consent))
	r.With(h.guard.Require("customer:merge")).Get("/customers/duplicates", httpx.JSON(h.log, h.duplicates))
	r.With(h.guard.Require("customer:merge")).Post("/customers/merge", httpx.JSON(h.log, h.merge))
	r.With(h.guard.Require("data_request:manage")).Post("/customers/{id}/anonymize", httpx.JSON(h.log, h.anonymize))
}

// anonymize atiende la solicitud de borrado del titular (HU-87): no se puede deshacer.
func (h *Handler) anonymize(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrCustomerNotFound)
	if err != nil {
		return 0, nil, err
	}
	out, err := h.svc.Anonymize(r.Context(), p.TallerID, p.Actor(), id)
	return http.StatusOK, map[string]any{"anonimizados": out}, err
}

// duplicates lista los posibles repetidos agrupados por nombre (HU-15).
func (h *Handler) duplicates(r *http.Request, p reqctx.Principal) (int, any, error) {
	groups, err := h.svc.Duplicates(r.Context(), p.TallerID)
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		cs := make([]customerDTO, 0, len(g.Customers))
		for _, c := range g.Customers {
			cs = append(cs, toDTO(c))
		}
		out = append(out, map[string]any{"clientes": cs})
	}
	return http.StatusOK, map[string]any{"grupos": out}, err
}

// merge une el cliente "unir" al que se "conservar" (HU-15): sus vehículos, órdenes y conversaciones pasan al que queda.
func (h *Handler) merge(r *http.Request, p reqctx.Principal) (int, any, error) {
	var q struct {
		Conservar string `json:"conservar"`
		Unir      string `json:"unir"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	keep, err := shared.ParseID(q.Conservar)
	if err != nil {
		return 0, nil, domain.ErrCustomerNotFound
	}
	drop, err := shared.ParseID(q.Unir)
	if err != nil {
		return 0, nil, domain.ErrCustomerNotFound
	}
	res, err := h.svc.Merge(r.Context(), p.TallerID, p.Actor(), keep, drop)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{"cliente": toDTO(res.Kept), "movidos": res.Moved}, nil
}

func (h *Handler) list(r *http.Request, p reqctx.Principal) (int, any, error) {
	var list []domain.Customer
	var err error
	if q := r.URL.Query().Get("q"); q != "" {
		list, err = h.svc.Search(r.Context(), p.TallerID, q)
	} else {
		list, err = h.svc.List(r.Context(), p.TallerID)
	}
	out := make([]customerDTO, 0, len(list))
	for _, c := range list {
		out = append(out, toDTO(c))
	}
	return http.StatusOK, map[string]any{"clientes": out}, err
}

// consent registra la autorización dada en el mostrador (HU-86): el taller muestra el texto y el cliente acepta.
func (h *Handler) consent(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrCustomerNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Acepta bool `json:"acepta"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	by := p.UserID
	if err := h.svc.SetConsent(r.Context(), p.TallerID, id, p.Actor(), q.Acepta, app.ConsentRecord{
		Text: ConsentText, Version: ConsentVersion, Channel: "mostrador", At: time.Now().UTC(), By: &by}); err != nil {
		return 0, nil, err
	}
	c, err := h.svc.ByID(r.Context(), p.TallerID, id)
	return http.StatusOK, toDTO(c), err
}
