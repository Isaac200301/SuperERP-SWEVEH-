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
	"sweveh/internal/messaging/adapters/out/whatsapp/waha"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
)

// Public monta /webhooks/whatsapp (Meta Cloud API, firma HMAC SHA-256) y /webhooks/waha (demo, SHA-512): sin
// cookies ni CSRF (R-05, R-09).
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
			receive(w, r, body, meta.ValidSignature(cfg.WhatsApp.AppSecret, body, r.Header.Get(meta.SignatureHeader)), svc, log)
		})
		// WAHA (demo, ADR-0027): firma HMAC-SHA512 con WAHA_WEBHOOK_SECRET. Sin WAHA configurado, la firma nunca
		// valida y todo se rechaza.
		r.Post("/webhooks/waha", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			ok := cfg.WhatsApp.WAHA.Enabled() && waha.ValidSignature(cfg.WhatsApp.WAHA.WebhookSecret, body, r.Header.Get(waha.SignatureHeader))
			receive(w, r, body, ok, svc, log)
		})
	}
}

// receive guarda el cuerpo crudo si la firma es válida; el worker lo interpreta después.
func receive(w http.ResponseWriter, r *http.Request, body []byte, signed bool, svc *messagingapp.Service, log *slog.Logger) {
	if !signed {
		log.WarnContext(r.Context(), "webhook con firma inválida", "ruta", r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if err := svc.ReceiveWebhook(r.Context(), body); err != nil {
		log.ErrorContext(r.Context(), "no se pudo guardar el webhook", "error", err)
		w.WriteHeader(http.StatusInternalServerError) // el proveedor reintenta
		return
	}
	w.WriteHeader(http.StatusOK)
}
