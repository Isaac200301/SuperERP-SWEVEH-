package authn

import (
	"log/slog"
	"net/http"
	"time"

	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Nombres de cookie de docs/06 §6.3. __Host- obliga Secure, Path=/ y prohíbe Domain.
// El refresh usa __Secure- para limitarlo a /api/v1/auth (refresh y logout).
const (
	AccessCookie  = "__Host-sw_at"
	RefreshCookie = "__Secure-sw_rt"
	RefreshPath   = "/api/v1/auth"
)

// SetSessionCookies escribe las dos cookies HttpOnly + Secure + SameSite=Strict.
func SetSessionCookies(w http.ResponseWriter, access AccessToken, refresh string, refreshExp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: AccessCookie, Value: access.Value, Path: "/", Expires: access.ExpiresAt,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: RefreshCookie, Value: refresh, Path: RefreshPath, Expires: refreshExp,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
}

// ClearSessionCookies vence las dos cookies.
func ClearSessionCookies(w http.ResponseWriter) {
	for _, c := range []struct{ name, path string }{{AccessCookie, "/"}, {RefreshCookie, RefreshPath}} {
		http.SetCookie(w, &http.Cookie{
			Name: c.name, Value: "", Path: c.path, MaxAge: -1,
			HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
		})
	}
}

// PermissionChecker decide si unos roles tienen un permiso (lo inyecta el composition root
// desde identity, para que platform no dependa de un módulo).
type PermissionChecker func(roles []string, perm string) bool

// Guard autentica y autoriza las rutas del realm taller.
type Guard struct {
	verifier *Verifier
	can      PermissionChecker
	log      *slog.Logger
}

func NewGuard(v *Verifier, can PermissionChecker, log *slog.Logger) *Guard {
	return &Guard{verifier: v, can: can, log: log}
}

var ErrForbidden = shared.Forbidden("sin_permiso", "Tu rol no permite hacer esto.")

// Can responde si el usuario de la solicitud tiene el permiso (para ocultar campos, p. ej. costos, RF-21).
func (g *Guard) Can(p reqctx.Principal, perm string) bool { return g.can(p.Roles, perm) }

// Authenticate lee la cookie de acceso, valida el JWT y pone el Principal (con su taller) en el context.
func (g *Guard) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(AccessCookie)
		if err != nil || c.Value == "" {
			httpx.WriteError(w, r, g.log, ErrInvalidToken)
			return
		}
		claims, err := g.verifier.Verify(c.Value)
		if err != nil {
			httpx.WriteError(w, r, g.log, err)
			return
		}
		uid, err1 := shared.ParseID(claims.Subject)
		tid, err2 := shared.ParseID(claims.TallerID)
		sid, err3 := shared.ParseID(claims.SessionID)
		if err1 != nil || err2 != nil || err3 != nil {
			httpx.WriteError(w, r, g.log, ErrInvalidToken)
			return
		}
		p := reqctx.Principal{UserID: uid, TallerID: tid, SessionID: sid, Roles: claims.Roles}
		next.ServeHTTP(w, r.WithContext(reqctx.WithPrincipal(r.Context(), p)))
	})
}

// Require exige un permiso `recurso:accion` (deny by default). Va después de Authenticate.
func (g *Guard) Require(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := reqctx.PrincipalFrom(r.Context())
			if !ok {
				httpx.WriteError(w, r, g.log, ErrInvalidToken)
				return
			}
			if !g.can(p.Roles, perm) {
				g.log.WarnContext(r.Context(), "permiso denegado", "permiso", perm, "ruta", r.URL.Path)
				httpx.WriteError(w, r, g.log, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
