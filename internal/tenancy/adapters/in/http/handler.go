// Package http expone la configuración del taller (HU-03, HU-04, HU-08, HU-89).
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/tenancy"
	"sweveh/internal/tenancy/domain"
)

type Handler struct {
	admin *tenancy.Admin
	guard *authn.Guard
	log   *slog.Logger
}

func NewHandler(a *tenancy.Admin, g *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{admin: a, guard: g, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/workshop", httpx.JSON(h.log, h.get))
	r.With(h.guard.Require("workshop:manage")).Post("/workshop/onboarding", httpx.JSON(h.log, h.onboard))
	r.With(h.guard.Require("workshop:manage")).Put("/workshop/schedule", httpx.JSON(h.log, h.schedule))
}

type dto struct {
	ID                 string          `json:"id"`
	Nombre             string          `json:"nombre"`
	Atiende            string          `json:"atiende"`
	Especialidades     []string        `json:"especialidades"`
	Ciudad             string          `json:"ciudad"`
	Tamano             string          `json:"tamano"`
	Horario            domain.Schedule `json:"horario"`
	ContratoVersion    *string         `json:"contratoVersion"`
	ContratoAceptadoEn *time.Time      `json:"contratoAceptadoEn"`
	OnboardingCompleto bool            `json:"onboardingCompleto"`
}

func toDTO(s tenancy.Settings) dto {
	d := dto{ID: s.ID.String(), Nombre: s.Name, Atiende: s.Profile.Attends, Especialidades: s.Profile.Specialties, Ciudad: s.Profile.City,
		Tamano: s.Profile.Size, Horario: s.Schedule, ContratoVersion: s.ContractVersion, ContratoAceptadoEn: s.ContractAcceptedAt,
		OnboardingCompleto: s.Onboarded}
	if d.Especialidades == nil {
		d.Especialidades = []string{}
	}
	if d.Horario == nil {
		d.Horario = domain.Schedule{}
	}
	return d
}

func (h *Handler) get(r *http.Request, p reqctx.Principal) (int, any, error) {
	s, err := h.admin.Get(r.Context(), p.TallerID)
	return http.StatusOK, toDTO(s), err
}

type onboardReq struct {
	Nombre           string   `json:"nombre"`
	Atiende          string   `json:"atiende"`
	Especialidades   []string `json:"especialidades"`
	Ciudad           string   `json:"ciudad"`
	Tamano           string   `json:"tamano"`
	ContratoAceptado bool     `json:"contratoAceptado"`
}

func (h *Handler) onboard(r *http.Request, p reqctx.Principal) (int, any, error) {
	var q onboardReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	s, err := h.admin.Onboard(r.Context(), p.TallerID, p.Actor(), tenancy.OnboardInput{Name: q.Nombre, ContractAccepted: q.ContratoAceptado,
		Profile: domain.Profile{Attends: q.Atiende, Specialties: q.Especialidades, City: q.Ciudad, Size: q.Tamano}})
	return http.StatusOK, toDTO(s), err
}

func (h *Handler) schedule(r *http.Request, p reqctx.Principal) (int, any, error) {
	var q struct {
		Horario domain.Schedule `json:"horario"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	s, err := h.admin.SetSchedule(r.Context(), p.TallerID, p.Actor(), q.Horario)
	return http.StatusOK, toDTO(s), err
}
