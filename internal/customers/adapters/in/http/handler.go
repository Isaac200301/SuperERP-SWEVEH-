// Package http expone los casos de uso de customers. Solo traduce, autoriza y llama al caso de uso.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/customers/app"
	"sweveh/internal/customers/domain"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Permisos que usan estas rutas (los define identity; aquí solo se nombran).
const (
	permCreate = "customer:create"
	permRead   = "customer:read_all"
)

type Handler struct {
	svc   *app.Service
	guard *authn.Guard
	log   *slog.Logger
}

func NewHandler(svc *app.Service, guard *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{svc: svc, guard: guard, log: log}
}

// Routes monta las rutas bajo /api/v1 (el router ya pasó por Authenticate).
func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require(permCreate)).Post("/customers", h.register)
	// Buscar por celular es parte de "buscar o crear" (HU-09): lo puede hacer quien puede crear.
	r.With(h.guard.Require(permCreate)).Get("/customers/by-phone/{celular}", h.byPhone)
	r.With(h.guard.Require(permRead)).Get("/customers/{id}", h.byID)
}

type customerDTO struct {
	ID                 string    `json:"id"`
	Nombre             string    `json:"nombre"`
	Celular            string    `json:"celular"`
	Correo             *string   `json:"correo"`
	AutorizacionEstado string    `json:"autorizacionEstado"`
	CreadoEn           time.Time `json:"creadoEn"`
}

func toDTO(c domain.Customer) customerDTO {
	return customerDTO{
		ID: c.ID.String(), Nombre: c.Name, Celular: shared.NationalNumber(c.Phone), Correo: c.Email,
		AutorizacionEstado: string(c.Consent), CreadoEn: c.CreatedAt,
	}
}

type registerReq struct {
	Celular string  `json:"celular"`
	Nombre  string  `json:"nombre"`
	Correo  *string `json:"correo"`
}

type registerResp struct {
	Cliente   customerDTO `json:"cliente"`
	Existente bool        `json:"existente"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	var req registerReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	res, err := h.svc.Register(r.Context(), app.RegisterCustomer{
		TallerID: p.TallerID, Actor: p.Actor(), Phone: req.Celular, Name: req.Nombre, Email: req.Correo,
	})
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	status := http.StatusCreated
	if res.Existing {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, registerResp{Cliente: toDTO(res.Customer), Existente: res.Existing})
}

func (h *Handler) byPhone(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	c, err := h.svc.ByPhone(r.Context(), p.TallerID, chi.URLParam(r, "celular"))
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toDTO(c))
}

func (h *Handler) byID(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.log, domain.ErrCustomerNotFound)
		return
	}
	c, err := h.svc.ByID(r.Context(), p.TallerID, id)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toDTO(c))
}
