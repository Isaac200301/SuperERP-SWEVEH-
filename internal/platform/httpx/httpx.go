// Package httpx reúne lo transversal del borde HTTP: JSON, errores RFC 9457,
// cabeceras de seguridad, CSRF, request_id y límites de tasa (docs/06 §6.4, §7.2, §7.3).
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// MaxBodyBytes limita el cuerpo de las solicitudes JSON (valor inicial, a calibrar).
const MaxBodyBytes = 64 << 10

// ErrBadJSON: el cuerpo no es el JSON esperado.
var ErrBadJSON = shared.Validation("json_invalido", "No pudimos leer los datos enviados.")

// WriteJSON responde con JSON y el código dado.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON lee el cuerpo con tamaño máximo y sin campos desconocidos.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes) // w puede ser nil
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrBadJSON
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrBadJSON
	}
	return nil
}

// Problem es el cuerpo application/problem+json (RFC 9457).
type Problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	Detail    string `json:"detail"`
	Field     string `json:"field,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteError traduce un error a problem+json. Los errores desconocidos responden 500
// sin detalles internos y se registran con el request_id.
func WriteError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	p := Problem{RequestID: reqctx.RequestIDFrom(r.Context())}
	if e, ok := shared.AsError(err); ok {
		p.Status = statusOf(e.Kind)
		p.Code = e.Code
		p.Detail = e.Message
		p.Field = e.Field
	} else {
		log.ErrorContext(r.Context(), "error interno", "error", err.Error(), "ruta", r.URL.Path)
		p.Status = http.StatusInternalServerError
		p.Code = "error_interno"
		p.Detail = "Algo falló de nuestro lado. Intenta de nuevo en un momento."
	}
	p.Title = http.StatusText(p.Status)
	p.Type = "https://sweveh.app/errores/" + strings.ReplaceAll(p.Code, "_", "-")
	if p.Status == http.StatusTooManyRequests && w.Header().Get("Retry-After") == "" {
		w.Header().Set("Retry-After", "60")
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

func statusOf(k shared.Kind) int {
	switch k {
	case shared.KindValidation:
		return http.StatusBadRequest
	case shared.KindUnauthorized:
		return http.StatusUnauthorized
	case shared.KindForbidden:
		return http.StatusForbidden
	case shared.KindNotFound:
		return http.StatusNotFound
	case shared.KindConflict:
		return http.StatusConflict
	case shared.KindRule:
		return http.StatusUnprocessableEntity
	case shared.KindRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// ETag de un recurso versionado (concurrencia optimista, R-07).
func ETag(version int) string { return fmt.Sprintf(`"v%d"`, version) }

// ErrPreconditionRequired: falta If-Match en una edición.
var ErrPreconditionRequired = shared.Validation("if_match_requerido",
	"Falta la versión del registro (encabezado If-Match). Recarga e intenta de nuevo.")

// IfMatchVersion lee If-Match: "v3" → 3.
func IfMatchVersion(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.Header.Get("If-Match"))
	raw = strings.TrimPrefix(raw, "W/")
	var v int
	if _, err := fmt.Sscanf(raw, `"v%d"`, &v); err != nil || v < 1 {
		return 0, ErrPreconditionRequired
	}
	return v, nil
}
