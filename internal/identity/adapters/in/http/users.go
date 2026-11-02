package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/identity/app"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// UserRoutes: gestión de usuarios (HU-80, HU-82, HU-84) y cambio de taller.
func (h *Handler) UserRoutes(r chi.Router, guard *authn.Guard) {
	r.With(guard.Require("user:manage")).Get("/users", httpx.JSON(h.log, h.listUsers))
	r.With(guard.Require("user:manage")).Post("/users", httpx.JSON(h.log, h.addUser))
	r.With(guard.Require("user:manage")).Patch("/users/{id}", httpx.JSON(h.log, h.setUserActive))
	r.Get("/me/workshops", httpx.JSON(h.log, h.myWorkshops))
	r.Post("/auth/switch", h.switchWorkshop)
}

type memberDTO struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"nombre"`
	Celular  string    `json:"celular"`
	Rol      string    `json:"rol"`
	Activo   bool      `json:"activo"`
	CreadoEn time.Time `json:"creadoEn"`
}

func memberOut(m app.Member) memberDTO {
	return memberDTO{ID: m.UserID.String(), Nombre: m.Name, Celular: shared.NationalNumber(m.Phone), Rol: m.Role, Activo: m.Active, CreadoEn: m.CreatedAt}
}

func (h *Handler) listUsers(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.svc.ListMembers(r.Context(), p.TallerID)
	out := make([]memberDTO, 0, len(list))
	for _, m := range list {
		out = append(out, memberOut(m))
	}
	return http.StatusOK, map[string]any{"usuarios": out}, err
}

func (h *Handler) addUser(r *http.Request, p reqctx.Principal) (int, any, error) {
	var q struct {
		Celular string `json:"celular"`
		Nombre  string `json:"nombre"`
		Rol     string `json:"rol"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	m, err := h.svc.AddMember(r.Context(), p.TallerID, p.Actor(), q.Celular, q.Nombre, q.Rol)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, memberOut(m), nil
}

func (h *Handler) setUserActive(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", app.ErrMemberNF)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Activo bool `json:"activo"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	m, err := h.svc.SetMemberActive(r.Context(), p.TallerID, p.Actor(), id, q.Activo)
	return http.StatusOK, memberOut(m), err
}

func (h *Handler) myWorkshops(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.svc.Workshops(r.Context(), p.UserID)
	out := make([]workshopDTO, 0, len(list))
	for _, w := range list {
		out = append(out, workshopDTO{ID: w.ID.String(), Nombre: w.Name})
	}
	return http.StatusOK, map[string]any{"talleres": out, "actual": p.TallerID.String()}, err
}

func (h *Handler) switchWorkshop(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	var q struct {
		TallerID string `json:"tallerId"`
	}
	if err := httpx.DecodeJSON(w, r, &q); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	tid, err := shared.ParseID(q.TallerID)
	if err != nil {
		httpx.WriteError(w, r, h.log, app.ErrNotMember)
		return
	}
	s, err := h.svc.Switch(r.Context(), p.UserID, p.SessionID, tid)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	h.writeSession(w, s, http.StatusOK)
}
