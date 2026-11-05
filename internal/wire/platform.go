package wire

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"sweveh/internal/platform/db"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

// Medidor del administrador del ERP (HU-74, ADR-0030): mensajes enviados y costo de IA del mes por taller. Solo lo ve
// quien está en PLATFORM_ADMINS; a un usuario de taller (dueño, mecánico o recepción) le responde "sin permiso".

var errNotPlatformAdmin = shared.Forbidden("sin_permiso", "No tienes permiso para ver esto.")

type usageRow struct {
	ID, Name             string
	Messages, Included   int
	AIMicroUSD, AICOP    int64
	MessagesText, AIText string
	Over                 bool
}

// requireAdmin responde "sin permiso" a quien no está en PLATFORM_ADMINS (ADR-0030).
func (h *handler) requireAdmin(r *http.Request, p reqctx.Principal) error {
	if h.s.M.PlatformAdmin == nil {
		return errNotPlatformAdmin
	}
	ok, err := h.s.M.PlatformAdmin(r.Context(), p)
	if err != nil {
		return err
	}
	if !ok {
		return errNotPlatformAdmin
	}
	return nil
}

func (h *handler) usage(r *http.Request, p reqctx.Principal) (int, any, error) {
	if err := h.requireAdmin(r, p); err != nil {
		return 0, nil, err
	}
	rows, err := h.platformUsage(r.Context())
	if err != nil {
		return 0, nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, x := range rows {
		out = append(out, map[string]any{"id": x.ID, "nombre": x.Name, "mensajes": x.Messages, "incluidos": x.Included,
			"mensajesTexto": x.MessagesText, "iaMicroUsd": x.AIMicroUSD, "iaPesos": x.AICOP, "iaTexto": x.AIText, "sePasa": x.Over})
	}
	now := h.s.M.Clock.Now()
	return http.StatusOK, map[string]any{"talleres": out, "mes": now.Format("2006-01"),
		"incluidos": h.cfg.Platform.IncludedMessages, "pesosPorDolar": h.cfg.Platform.COPPerUSD}, nil
}

type workshopRow struct {
	ID     shared.ID `db:"id"`
	Nombre string    `db:"nombre"`
}

// workshops lista los talleres para la plataforma (excepción controlada de ADR-0030: solo id y nombre).
func (h *handler) workshops(ctx context.Context) ([]workshopRow, error) {
	var list []workshopRow
	err := h.s.M.Tx.DoWithSettings(ctx, map[string]string{"app.admin_plataforma": "on"}, func(ctx context.Context) error {
		var err error
		list, err = db.Many[workshopRow](ctx, `SELECT id, nombre FROM tenancy.taller ORDER BY nombre`)
		return err
	})
	return list, err
}

// webhookDays es cuánto atrás busca el registro técnico en los webhooks crudos.
const webhookDays = 7

// messageLog es el registro técnico de mensajes (HU-100): por celular, qué entró y salió en cada taller con su estado
// (pendiente, enviado, entregado, leído o fallido), el error y la hora, y los webhooks crudos de los últimos 7 días que
// lo mencionan (si llegaron y si se procesaron). No muestra el texto de los mensajes.
func (h *handler) messageLog(r *http.Request, p reqctx.Principal) (int, any, error) {
	if err := h.requireAdmin(r, p); err != nil {
		return 0, nil, err
	}
	phone, err := shared.ParseColombianMobile(r.URL.Query().Get("celular"))
	if err != nil {
		return 0, nil, err
	}
	ctx := r.Context()
	list, err := h.workshops(ctx)
	if err != nil {
		return 0, nil, err
	}
	msgs := []map[string]any{}
	for _, t := range list {
		ms, err := h.s.Messaging.MessagesByPhone(reqctx.WithTenant(ctx, t.ID), t.ID, phone)
		if err != nil {
			return 0, nil, err
		}
		for _, m := range ms {
			msgs = append(msgs, map[string]any{"taller": t.Nombre, "fecha": m.CreatedAt, "direccion": m.Direction, "tipo": m.Kind,
				"estado": m.State, "origen": m.Origin, "error": m.Error, "enviadoEn": m.SentAt, "wamid": m.WAMID})
		}
	}
	sort.SliceStable(msgs, func(i, j int) bool { return msgs[i]["fecha"].(time.Time).After(msgs[j]["fecha"].(time.Time)) })
	hooks, err := h.s.Messaging.WebhooksMentioning(ctx, phone[1:], h.s.M.Clock.Now().AddDate(0, 0, -webhookDays))
	if err != nil {
		return 0, nil, err
	}
	wh := make([]map[string]any, 0, len(hooks))
	for _, x := range hooks {
		wh = append(wh, map[string]any{"id": x.ID, "recibido": x.ReceivedAt, "procesado": x.ProcessedAt, "error": x.Error})
	}
	return http.StatusOK, map[string]any{"celular": shared.NationalNumber(phone), "mensajes": msgs, "webhooks": wh,
		"diasWebhooks": webhookDays}, nil
}

// platformUsage lista los talleres (excepción controlada de ADR-0030: solo id y nombre, con app.admin_plataforma) y
// lee las cifras de cada uno dentro de su propio taller, con el aislamiento normal.
func (h *handler) platformUsage(ctx context.Context) ([]usageRow, error) {
	list, err := h.workshops(ctx)
	if err != nil {
		return nil, err
	}
	now := h.s.M.Clock.Now()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	included, rate := h.cfg.Platform.IncludedMessages, h.cfg.Platform.COPPerUSD
	out := make([]usageRow, 0, len(list))
	for _, t := range list {
		tctx := reqctx.WithTenant(ctx, t.ID)
		msgs, err := h.s.Messaging.SentSince(tctx, t.ID, since)
		if err != nil {
			return nil, err
		}
		cost, err := h.s.Assistant.MonthCost(tctx, t.ID)
		if err != nil {
			return nil, err
		}
		cop := int64(math.Round(float64(cost) * float64(rate) / 1e6))
		out = append(out, usageRow{ID: t.ID.String(), Name: t.Nombre, Messages: msgs, Included: included, AIMicroUSD: cost, AICOP: cop,
			MessagesText: fmt.Sprintf("%s de %s", thousands(int64(msgs)), thousands(int64(included))), AIText: "IA $" + thousands(cop),
			Over: msgs > included})
	}
	// Primero el que más cuesta: así se ve al que se pasa (HU-74, criterio 2).
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AICOP != out[j].AICOP {
			return out[i].AICOP > out[j].AICOP
		}
		return out[i].Messages > out[j].Messages
	})
	return out, nil
}

// thousands escribe el número con punto de miles: 1200 → "1.200".
func thousands(v int64) string {
	s := fmt.Sprintf("%d", v)
	out := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	return out
}
