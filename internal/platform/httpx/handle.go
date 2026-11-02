package httpx

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Func es un handler que devuelve estado, cuerpo y error; JSON lo escribe de forma uniforme.
type Func func(r *http.Request, p reqctx.Principal) (int, any, error)

// JSON adapta un Func: errores en problem+json, cuerpos en JSON, 204 sin cuerpo.
func JSON(log *slog.Logger, fn Func) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, _ := reqctx.PrincipalFrom(r.Context())
		status, body, err := fn(r, p)
		if err != nil {
			WriteError(w, r, log, err)
			return
		}
		if status == http.StatusNoContent || body == nil {
			w.WriteHeader(status)
			return
		}
		WriteJSON(w, status, body)
	}
}

// Decode lee el cuerpo JSON estricto.
func Decode(r *http.Request, dst any) error { return DecodeJSON(nil, r, dst) }

// PathID lee un UUID de la ruta; si no es válido responde el NotFound del recurso (no revela nada).
func PathID(r *http.Request, name string, notFound *shared.Error) (shared.ID, error) {
	id, err := shared.ParseID(chi.URLParam(r, name))
	if err != nil {
		return shared.NilID, notFound
	}
	return id, nil
}
