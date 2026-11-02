package httpx

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// RequestID agrega un identificador por solicitud (lo devuelve en X-Request-ID).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b [12]byte
		_, _ = rand.Read(b[:])
		id := hex.EncodeToString(b[:])
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(reqctx.WithRequestID(r.Context(), id)))
	})
}

// Recover convierte un panic en 500 sin filtrar detalles.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					log.ErrorContext(r.Context(), "panic en solicitud", "valor", v, "ruta", r.URL.Path)
					WriteError(w, r, log, errPanic)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

var errPanic = &panicErr{}

type panicErr struct{}

func (*panicErr) Error() string { return "panic" }

// AccessLog registra método, ruta, estado y duración. Nunca cuerpos, cookies ni encabezados.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.InfoContext(r.Context(), "http",
				"metodo", r.Method, "ruta", r.URL.Path, "status", sw.status,
				"duration_ms", time.Since(start).Milliseconds())
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// SecurityHeaders aplica las cabeceras de docs/06 §7.2 y la skill sweveh-seguridad-auth §6.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			// Fuentes autoalojadas: la CSP no abre ninguna excepción a terceros.
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; "+
				"img-src 'self' data: blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", "camera=(self), geolocation=(), microphone=()")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("X-Frame-Options", "DENY")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRFHeader es el encabezado personalizado obligatorio en métodos que cambian estado.
const CSRFHeader = "X-Requested-With"
const CSRFValue = "sweveh"

var ErrCSRF = shared.Forbidden("csrf", "La solicitud no viene de la aplicación de SWEVEH.")

// CSRF exige el encabezado personalizado y un Origin permitido (o Sec-Fetch-Site same-origin)
// en POST/PUT/PATCH/DELETE (docs/06 §6.4). Las rutas /webhooks/* no pasan por aquí.
func CSRF(allowedOrigins []string, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get(CSRFHeader) != CSRFValue {
				WriteError(w, r, log, ErrCSRF)
				return
			}
			origin := r.Header.Get("Origin")
			site := r.Header.Get("Sec-Fetch-Site")
			switch {
			case origin != "" && slices.Contains(allowedOrigins, origin):
			case origin == "" && (site == "same-origin" || site == ""):
				// Sin Origin (algunas solicitudes same-origin): se acepta solo si el navegador
				// no dice que viene de otro sitio. El encabezado personalizado ya exige CORS.
			default:
				WriteError(w, r, log, ErrCSRF)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP devuelve la IP del cliente. Solo confía en X-Forwarded-For si trustProxy es true
// (detrás del proxy de la plataforma); si no, usa la conexión.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			return strings.TrimSpace(first)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
