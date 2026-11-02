package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/customers/app"
	"sweveh/internal/customers/domain"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
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
