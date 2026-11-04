package wire

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	messagingpg "sweveh/internal/messaging/adapters/out/postgres"
	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
)

// Public monta /webhooks/whatsapp (Meta Cloud API): sin cookies ni CSRF, con firma HMAC SHA-256 (R-05, R-09).
// Solo guarda el cuerpo crudo y responde rápido; el worker lo interpreta después con el parser del proveedor.
func Public(pool *pgxpool.Pool, cfg config.API, log *slog.Logger) func(chi.Router) {
	svc := messagingapp.NewService(db.NewTxManager(pool), messagingpg.Repository{}, nil, nil, nil, clock.System{})
	return func(r chi.Router) {
		r.Get("/webhooks/whatsapp", func(w http.ResponseWriter, r *http.Request) {
			challenge, ok := meta.VerifyChallenge(cfg.WhatsApp.VerifyToken, r.URL.Query())
			if !ok {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(strconv.FormatUint(challenge, 10)))
		})
		r.Post("/webhooks/whatsapp", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // cuerpo crudo: la firma se calcula sobre estos bytes
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !meta.ValidSignature(cfg.WhatsApp.AppSecret, body, r.Header.Get(meta.SignatureHeader)) {
				log.WarnContext(r.Context(), "webhook con firma inválida")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if err := svc.ReceiveWebhook(r.Context(), body); err != nil {
				log.ErrorContext(r.Context(), "no se pudo guardar el webhook", "error", err)
				w.WriteHeader(http.StatusInternalServerError) // Meta reintenta
				return
			}
			w.WriteHeader(http.StatusOK)
		})
	}
}
