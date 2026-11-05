package demo

import (
	"io"
	"log/slog"
	"net/http"
	"time"

	"sweveh/internal/platform/clock"
)

// AsOwner entra una vez como el dueño de la demo y lee esas rutas de la API (lo que ve en el panel). El reloj va unos
// minutos adelante para no chocar con el límite de un código por minuto de la carga.
func AsOwner(d Deps, paths ...string) ([]map[string]any, error) {
	if d.Log == nil {
		d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	l := newLoader(d, clock.NewFake(time.Now().Add(3*time.Minute)))
	if err := l.login(d.Phones.Owner); err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, p := range paths {
		r, err := l.call(http.StatusOK, "GET", p, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
