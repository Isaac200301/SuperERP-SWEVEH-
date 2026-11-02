// Package http expone el inicio de sesión por celular (HU-01, HU-02) y la sesión actual.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/identity/app"
	"sweveh/internal/identity/domain"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

var ErrTooManyRequests = shared.RateLimited("demasiadas_solicitudes", "Hiciste muchos intentos seguidos. Espera un minuto.")

type Handler struct {
	svc        *app.Service
	log        *slog.Logger
	limiter    *httpx.RateLimiter
	trustProxy bool
}

func NewHandler(svc *app.Service, log *slog.Logger, limiter *httpx.RateLimiter, trustProxy bool) *Handler {
	return &Handler{svc: svc, log: log, limiter: limiter, trustProxy: trustProxy}
}

// PublicRoutes no exigen sesión.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.With(h.limit).Post("/auth/otp", h.requestCode)
	r.With(h.limit).Post("/auth/otp/verify", h.verify)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
}

// PrivateRoutes van detrás de Authenticate.
func (h *Handler) PrivateRoutes(r chi.Router) {
	r.Get("/me", h.me)
}

// limit aplica el límite por IP en login (más estricto que el resto, skill sweveh-seguridad-auth §5).
func (h *Handler) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.limiter.Allow(httpx.ClientIP(r, h.trustProxy)) {
			httpx.WriteError(w, r, h.log, ErrTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type requestCodeReq struct {
	Celular string `json:"celular"`
}

type requestCodeResp struct {
	VenceEn time.Time `json:"venceEn"`
}

func (h *Handler) requestCode(w http.ResponseWriter, r *http.Request) {
	var req requestCodeReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	exp, err := h.svc.RequestCode(r.Context(), req.Celular)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	// 202 y el mismo cuerpo exista o no la cuenta: no revela qué celulares están registrados.
	httpx.WriteJSON(w, http.StatusAccepted, requestCodeResp{VenceEn: exp})
}

type verifyReq struct {
	Celular      string `json:"celular"`
	Codigo       string `json:"codigo"`
	Nombre       string `json:"nombre"`
	Autorizacion bool   `json:"autorizacionDatos"`
}

type sessionResp struct {
	Usuario     userDTO     `json:"usuario"`
	Taller      workshopDTO `json:"taller"`
	Roles       []string    `json:"roles"`
	CuentaNueva bool        `json:"cuentaNueva"`
	Permisos    []string    `json:"permisos"`
}

type userDTO struct {
	ID      string `json:"id"`
	Nombre  string `json:"nombre"`
	Celular string `json:"celular"`
}

type workshopDTO struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	var req verifyReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	s, err := h.svc.Verify(r.Context(), app.VerifyInput{
		Phone: req.Celular, Code: req.Codigo, Name: req.Nombre, ConsentAccepted: req.Autorizacion,
	})
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	h.writeSession(w, s, http.StatusOK)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(authn.RefreshCookie)
	if err != nil {
		authn.ClearSessionCookies(w)
		httpx.WriteError(w, r, h.log, authn.ErrInvalidToken)
		return
	}
	s, err := h.svc.Refresh(r.Context(), c.Value)
	if err != nil {
		authn.ClearSessionCookies(w)
		httpx.WriteError(w, r, h.log, err)
		return
	}
	h.writeSession(w, s, http.StatusOK)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	raw := ""
	if c, err := r.Cookie(authn.RefreshCookie); err == nil {
		raw = c.Value
	}
	if err := h.svc.Logout(r.Context(), raw); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	authn.ClearSessionCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	u, ws, err := h.svc.Me(r.Context(), p.UserID, p.TallerID)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sessionResp{
		Usuario: userDTO{ID: u.ID.String(), Nombre: u.Name, Celular: shared.NationalNumber(u.Phone)},
		Taller:  workshopDTO{ID: ws.ID.String(), Nombre: ws.Name},
		Roles:   p.Roles, Permisos: domain.Permissions(p.Roles),
	})
}

func (h *Handler) writeSession(w http.ResponseWriter, s app.Session, status int) {
	authn.SetSessionCookies(w, authn.AccessToken{Value: s.AccessToken, ExpiresAt: s.AccessExpires}, s.RefreshToken, s.RefreshExpires)
	w.Header().Set("Cache-Control", "no-store")
	httpx.WriteJSON(w, status, sessionResp{
		Usuario:     userDTO{ID: s.User.ID.String(), Nombre: s.User.Name, Celular: shared.NationalNumber(s.User.Phone)},
		Taller:      workshopDTO{ID: s.Workshop.ID.String(), Nombre: s.Workshop.Name},
		Roles:       s.Roles, Permisos: domain.Permissions(s.Roles),
		CuentaNueva: s.NewAccount,
	})
}
