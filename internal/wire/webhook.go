package wire

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	messagingpg "sweveh/internal/messaging/adapters/out/postgres"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/clock"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
)

// Public monta /webhooks/whatsapp: sin cookies ni CSRF, con firma HMAC SHA-256 (R-05, R-09).
func Public(pool *pgxpool.Pool, cfg config.API, log *slog.Logger) func(chi.Router) {
	svc := messagingapp.NewService(db.NewTxManager(pool), messagingpg.Repository{}, nil, nil, clock.System{})
	return func(r chi.Router) {
		r.Get("/webhooks/whatsapp", func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			// Verificación de Meta: devuelve el challenge solo si el token coincide (comparación en tiempo constante).
			if cfg.WhatsApp.VerifyToken == "" || q.Get("hub.mode") != "subscribe" ||
				subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(cfg.WhatsApp.VerifyToken)) != 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			// Meta manda un challenge numérico; se reescribe como número para no reflejar texto del cliente.
			n, err := strconv.ParseUint(q.Get("hub.challenge"), 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(strconv.FormatUint(n, 10)))
		})
		r.Post("/webhooks/whatsapp", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // cuerpo crudo: la firma se calcula sobre estos bytes
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !ValidSignature(cfg.WhatsApp.AppSecret, body, r.Header.Get("X-Hub-Signature-256")) {
				log.WarnContext(r.Context(), "webhook con firma inválida")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if err := svc.ReceiveWebhook(r.Context(), body); err != nil {
				log.ErrorContext(r.Context(), "no se pudo guardar el webhook", "error", err)
				w.WriteHeader(http.StatusInternalServerError) // Meta reintenta
				return
			}
			w.WriteHeader(http.StatusOK) // responder rápido; el worker procesa
		})
	}
}

// ValidSignature valida X-Hub-Signature-256 = "sha256=" + hex(HMAC_SHA256(appSecret, body)). Sin secreto, rechaza.
func ValidSignature(secret string, body []byte, header string) bool {
	if secret == "" || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	got, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}
