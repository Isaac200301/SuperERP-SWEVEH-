package wire

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/analytics"
	assistantapp "sweveh/internal/assistant/app"
	"sweveh/internal/audit"
	automationsdomain "sweveh/internal/automations/domain"
	"sweveh/internal/inventory"
	"sweveh/internal/messaging/adapters/out/whatsapp/local"
	"sweveh/internal/messaging/adapters/out/whatsapp/meta"
	"sweveh/internal/messaging/adapters/out/whatsapp/waha"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/server"
	"sweveh/internal/shared"
)

// Extra monta las rutas del motor en el api (mensajería, simulador, automatizaciones, bandeja, KPIs, bitácora).
func Extra(cfg config.API) func(m *server.Modules, g *authn.Guard) []func(chi.Router) {
	return func(m *server.Modules, g *authn.Guard) []func(chi.Router) {
		s := Build(m, cfg.WhatsApp, cfg.AI)
		h := &handler{s: s, g: g, log: m.Log, cfg: cfg}
		an := analytics.NewHandler(m.Tx, m.OrdersF, m.QuotesF, extra{s}, g, m.Log, m.Clock.Now)
		return []func(chi.Router){h.routes, an.Routes}
	}
}

type handler struct {
	s   *Services
	g   *authn.Guard
	log *slog.Logger
	cfg config.API
}

func (h *handler) routes(r chi.Router) {
	j := func(fn httpx.Func) http.HandlerFunc { return httpx.JSON(h.log, fn) }
	r.With(h.g.Require("conversation:read")).Get("/conversations", j(h.conversations))
	r.With(h.g.Require("conversation:read")).Get("/conversations/{id}", j(h.conversation))
	r.With(h.g.Require("inbox:manage")).Post("/conversations/{id}/messages", j(h.reply))
	r.With(h.g.Require("inbox:manage")).Patch("/conversations/{id}", j(h.mode))
	r.With(h.g.Require("order:read")).Get("/orders/{id}/messages", j(h.orderMessages))
	r.With(h.g.Require("workshop:manage")).Get("/whatsapp/channel", j(h.channel))
	r.With(h.g.Require("workshop:manage")).Put("/whatsapp/channel", j(h.setChannel))
	r.With(h.g.Require("workshop:manage")).Get("/whatsapp/waha", j(h.wahaStatus))
	r.With(h.g.Require("workshop:manage")).Post("/whatsapp/waha/start", j(h.wahaStart))
	r.With(h.g.Require("workshop:manage")).Post("/simulator/inbound", j(h.simulate))
	r.With(h.g.Require("order:read")).Get("/automations", j(h.cards))
	r.With(h.g.Require("automation:manage")).Patch("/automations/{code}", j(h.setCard))
	r.With(h.g.Require("automation:manage")).Get("/automations/runs", j(h.runs))
	r.With(h.g.Require("inbox:manage")).Get("/inbox", j(h.inbox))
	r.With(h.g.Require("inbox:manage")).Post("/inbox/{id}", j(h.resolve))
	r.With(h.g.Require("inbox:manage")).Get("/inbox/stats", j(h.inboxStats))
	r.With(h.g.Require("inbox:manage")).Get("/assistant/settings", j(h.assistantSettings))
	r.With(h.g.Require("automation:manage")).Put("/assistant/settings", j(h.setAssistantSettings))
	r.With(h.g.Require("audit:read")).Get("/audit", j(h.audit))
}

type msgDTO struct {
	ID        string          `json:"id"`
	Direccion string          `json:"direccion"`
	Tipo      string          `json:"tipo"`
	Cuerpo    string          `json:"cuerpo"`
	Payload   json.RawMessage `json:"payload"`
	Estado    string          `json:"estado"`
	Origen    string          `json:"origen"`
	OrdenID   *string         `json:"ordenId"`
	Automat   *string         `json:"automatizacion"`
	Error     *string         `json:"error"`
	CreadoEn  time.Time       `json:"creadoEn"`
}

func msgOut(m messagingapp.Message) msgDTO {
	d := msgDTO{ID: m.ID.String(), Direccion: m.Direction, Tipo: m.Kind, Cuerpo: m.Body, Payload: m.Payload, Estado: m.State,
		Origen: m.Origin, Automat: m.Automation, Error: m.Error, CreadoEn: m.CreatedAt}
	if m.OrderID != nil {
		s := m.OrderID.String()
		d.OrdenID = &s
	}
	return d
}

type convDTO struct {
	ID           string     `json:"id"`
	Celular      string     `json:"celular"`
	ClienteID    *string    `json:"clienteId"`
	Modo         string     `json:"modo"`
	PideHumano   bool       `json:"pideHumano"`
	VentanaHasta *time.Time `json:"ventanaAbiertaHasta"`
	Ultimo       string     `json:"ultimoMensaje,omitempty"`
	UltimoEn     *time.Time `json:"ultimoEn,omitempty"`
}

func convOut(c messagingapp.Conversation) convDTO {
	d := convDTO{ID: c.ID.String(), Celular: shared.NationalNumber(c.Phone), Modo: c.Mode, PideHumano: c.WantsHuman}
	if c.CustomerID != nil {
		s := c.CustomerID.String()
		d.ClienteID = &s
	}
	if c.LastInbound != nil {
		t := c.LastInbound.Add(24 * time.Hour)
		d.VentanaHasta = &t
	}
	return d
}

var errConv = shared.NotFound("conversacion_no_encontrada", "No encontramos esa conversación.")

func (h *handler) conversations(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.s.Messaging.Conversations(r.Context(), p.TallerID)
	out := make([]convDTO, 0, len(list))
	for _, c := range list {
		d := convOut(c.Conversation)
		at := c.LastAt
		d.Ultimo, d.UltimoEn = c.LastBody, &at
		out = append(out, d)
	}
	return http.StatusOK, map[string]any{"conversaciones": out}, err
}

func (h *handler) conversation(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errConv)
	if err != nil {
		return 0, nil, err
	}
	c, msgs, err := h.s.Messaging.Messages(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	out := make([]msgDTO, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msgOut(m))
	}
	return http.StatusOK, map[string]any{"conversacion": convOut(c), "mensajes": out}, nil
}

// reply: una persona del taller escribe directamente (Lo contesto yo, HU-59). Respeta la ventana de 24 h.
func (h *handler) reply(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errConv)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Texto   string  `json:"texto"`
		OrdenID *string `json:"ordenId"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	c, err := h.s.Messaging.ConversationByID(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	var oid *shared.ID
	if q.OrdenID != nil {
		if o, err := shared.ParseID(*q.OrdenID); err == nil {
			oid = &o
		}
	}
	m, err := h.s.Messaging.Enqueue(r.Context(), messagingapp.Outgoing{TallerID: p.TallerID, Phone: c.Phone, Text: q.Texto, Origin: "persona", OrderID: oid})
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, msgOut(m), nil
}

func (h *handler) mode(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", errConv)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Modo string `json:"modo"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.Modo != "agente" && q.Modo != "humano" {
		return 0, nil, shared.Validation("modo_invalido", "El modo es agente o humano.")
	}
	return http.StatusNoContent, nil, h.s.Messaging.SetMode(r.Context(), p.TallerID, id, q.Modo, false)
}

func (h *handler) orderMessages(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", shared.NotFound("orden_no_encontrada", "No encontramos esa orden en tu taller."))
	if err != nil {
		return 0, nil, err
	}
	if _, err := h.s.M.Flow.Get(r.Context(), p.TallerID, id); err != nil {
		return 0, nil, err
	}
	list, err := h.s.Messaging.ByOrder(r.Context(), p.TallerID, id)
	out := make([]msgDTO, 0, len(list))
	for _, m := range list {
		out = append(out, msgOut(m))
	}
	return http.StatusOK, map[string]any{"mensajes": out}, err
}

func (h *handler) channel(r *http.Request, p reqctx.Principal) (int, any, error) {
	ch, ok, err := h.s.Messaging.Channel(r.Context(), p.TallerID)
	if err != nil || !ok {
		return http.StatusOK, map[string]any{"canal": nil}, err
	}
	return http.StatusOK, map[string]any{"canal": map[string]any{"proveedor": ch.Provider, "numero": ch.Visible}}, nil
}

var errNoWAHA = shared.NotFound("waha_no_configurado", "El WhatsApp de prueba (WAHA) no está configurado en este servidor.")

// demoOnly: las rutas de demo (simulador y WAHA) no existen en producción.
func (h *handler) demoOnly() error {
	if h.cfg.Env == config.EnvProduction {
		return shared.NotFound("ruta_no_encontrada", "Esa ruta no existe.")
	}
	return nil
}

// setChannel cambia el número del taller entre el simulador ("prueba_local") y el WhatsApp de prueba de WAHA
// ("waha", ADR-0027), sin tocar nada más (HU-101). El número oficial de Meta se conecta aparte.
func (h *handler) setChannel(r *http.Request, p reqctx.Principal) (int, any, error) {
	if err := h.demoOnly(); err != nil {
		return 0, nil, err
	}
	var q struct {
		Proveedor string `json:"proveedor"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	ch := messagingapp.ChannelInfo{TallerID: p.TallerID}
	switch q.Proveedor {
	case "prueba_local":
		ch.Provider, ch.PhoneNumberID = "prueba_local", local.Prefix+p.TallerID.String()
	case "waha":
		if !h.cfg.WhatsApp.WAHA.Enabled() {
			return 0, nil, errNoWAHA
		}
		ch.Provider, ch.PhoneNumberID = "waha", waha.Prefix+h.cfg.WhatsApp.WAHA.Session
	default:
		return 0, nil, shared.Validation("proveedor_invalido", "Elige prueba_local o waha.").WithField("proveedor")
	}
	if err := h.s.Messaging.SetChannel(r.Context(), ch); err != nil {
		return 0, nil, err
	}
	return h.channel(r, p)
}

// wahaStatus dice si WAHA está configurado, el estado de la sesión y, si espera el escaneo, el QR como imagen
// (data URL). La clave de WAHA nunca sale al navegador.
func (h *handler) wahaStatus(r *http.Request, _ reqctx.Principal) (int, any, error) {
	if err := h.demoOnly(); err != nil {
		return 0, nil, err
	}
	w := h.cfg.WhatsApp.WAHA
	if !w.Enabled() {
		return http.StatusOK, map[string]any{"configurado": false}, nil
	}
	c := WAHAClient(w)
	st, err := c.SessionStatus(r.Context(), w.Session)
	if err != nil {
		h.log.WarnContext(r.Context(), "WAHA no responde", "error", err)
		return http.StatusOK, map[string]any{"configurado": true, "sesion": w.Session, "estado": "SIN_CONEXION"}, nil
	}
	out := map[string]any{"configurado": true, "sesion": w.Session, "estado": st.Status}
	if st.Me != nil {
		out["numero"] = st.Me.ID
	}
	if st.Status == "SCAN_QR_CODE" {
		if png, err := c.QR(r.Context(), w.Session); err == nil {
			out["qr"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		}
	}
	return http.StatusOK, out, nil
}

// wahaStart crea o arranca la sesión de WAHA para poder escanear el QR.
func (h *handler) wahaStart(r *http.Request, p reqctx.Principal) (int, any, error) {
	if err := h.demoOnly(); err != nil {
		return 0, nil, err
	}
	w := h.cfg.WhatsApp.WAHA
	if !w.Enabled() {
		return 0, nil, errNoWAHA
	}
	if err := WAHAClient(w).StartSession(r.Context(), w.Session); err != nil {
		h.log.WarnContext(r.Context(), "no se pudo arrancar la sesión de WAHA", "error", err)
		return 0, nil, shared.Rule("waha_sin_conexion", "No pudimos conectar con WAHA. Revisa que esté corriendo (make waha).")
	}
	return h.wahaStatus(r, p)
}

// simulate inyecta un mensaje de cliente por el canal de prueba local, con el mismo formato del webhook de Meta.
// Solo existe para la demo sin cuenta de Meta (D-01 pendiente) y nunca en producción.
func (h *handler) simulate(r *http.Request, p reqctx.Principal) (int, any, error) {
	if err := h.demoOnly(); err != nil {
		return 0, nil, err
	}
	var q struct {
		Celular string `json:"celular"`
		Nombre  string `json:"nombre"`
		Texto   string `json:"texto"`
		Boton   string `json:"boton"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	phone, err := shared.ParseColombianMobile(q.Celular)
	if err != nil {
		return 0, nil, shared.ErrPhoneInvalid.WithField("celular")
	}
	ch, err := h.s.Messaging.EnsureLocalChannel(r.Context(), p.TallerID)
	if err != nil {
		return 0, nil, err
	}
	// Con un número real (WAHA o Meta) las respuestas saldrían a ese celular: el simulador no se mezcla.
	if ch.Provider != "prueba_local" {
		return 0, nil, shared.Rule("canal_real", "El taller está usando un WhatsApp real. Vuelve al simulador en Mi taller para usar esta pantalla.")
	}
	body, err := meta.SimulatedBody(ch.PhoneNumberID, phone, q.Nombre, q.Texto, q.Boton, h.s.M.Clock.Now())
	if err != nil {
		return 0, nil, err
	}
	if err := h.s.Messaging.ReceiveWebhook(r.Context(), body); err != nil {
		return 0, nil, err
	}
	return http.StatusAccepted, map[string]any{"ok": true}, nil
}

func (h *handler) cards(r *http.Request, p reqctx.Principal) (int, any, error) {
	cards, err := h.s.Automations.Cards(r.Context(), p.TallerID)
	out := []map[string]any{}
	for _, c := range cards {
		out = append(out, map[string]any{"codigo": c.Code, "nombre": c.Name, "disparador": c.Trigger, "activa": c.Active,
			"proximamente": c.ComingSoon, "enviaMensaje": c.SendsMessage, "editable": c.Editable, "texto": c.Text,
			"datos": c.Placeholders, "ejecucionesSemana": c.WeekCount,
			"costo": map[bool]string{true: "1 mensaje por envío", false: "No cuesta mensajes"}[c.SendsMessage]})
	}
	return http.StatusOK, map[string]any{"automatizaciones": out}, err
}

func (h *handler) setCard(r *http.Request, p reqctx.Principal) (int, any, error) {
	code := chi.URLParam(r, "code")
	var q struct {
		Activa *bool                  `json:"activa"`
		Texto  httpx.Optional[string] `json:"texto"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.Activa != nil {
		if err := h.s.Automations.Toggle(r.Context(), p.TallerID, p.Actor(), code, *q.Activa); err != nil {
			return 0, nil, err
		}
	}
	if q.Texto.Set {
		if err := h.s.Automations.EditText(r.Context(), p.TallerID, p.Actor(), code, q.Texto.Value); err != nil {
			return 0, nil, err
		}
	}
	return h.cards(r, p)
}

func (h *handler) runs(r *http.Request, p reqctx.Principal) (int, any, error) {
	runs, err := h.s.Automations.Runs(r.Context(), p.TallerID)
	out := []map[string]any{}
	for _, x := range runs {
		out = append(out, map[string]any{"codigo": x.Code, "resultado": x.Result, "detalle": x.Detail, "ordenId": x.OrderID, "creadoEn": x.CreatedAt})
	}
	return http.StatusOK, map[string]any{"ejecuciones": out}, err
}

func draftOut(d assistantapp.DraftRecord) map[string]any {
	return map[string]any{"id": d.ID.String(), "conversacionId": d.ConversationID.String(), "celular": shared.NationalNumber(d.Phone),
		"mensajeCliente": d.InboundText, "intencion": d.Intent, "propuesto": d.Proposed, "final": d.Final, "fuentes": d.Sources,
		"accion": d.Action, "estado": d.State, "origen": d.Origin, "creadoEn": d.CreatedAt, "resueltoEn": d.ResolvedAt}
}

func (h *handler) inbox(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.s.Assistant.Drafts(r.Context(), p.TallerID, r.URL.Query().Get("todos") != "1")
	out := []map[string]any{}
	for _, d := range list {
		out = append(out, draftOut(d))
	}
	return http.StatusOK, map[string]any{"borradores": out}, err
}

func (h *handler) resolve(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", assistantapp.ErrDraftNF)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Decision string  `json:"decision"`
		Texto    *string `json:"texto"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	d, err := h.s.Assistant.Resolve(r.Context(), p.TallerID, id, p.Actor(), q.Decision, q.Texto)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, draftOut(d), nil
}

func (h *handler) inboxStats(r *http.Request, p reqctx.Principal) (int, any, error) {
	sent, edited, err := h.s.Assistant.Stats(r.Context(), p.TallerID)
	pct := 0.0
	if sent > 0 {
		pct = float64(edited) * 100 / float64(sent)
	}
	return http.StatusOK, map[string]any{"enviados": sent, "editados": edited, "porcentajeEditados": pct}, err
}

// assistantSettings dice si el agente responde solo (ADR-0028).
func (h *handler) assistantSettings(r *http.Request, p reqctx.Principal) (int, any, error) {
	on, err := h.s.Assistant.AutoReply(r.Context(), p.TallerID)
	return http.StatusOK, map[string]any{"autoEnvio": on}, err
}

// setAssistantSettings enciende o apaga el envío automático; solo el dueño (automation:manage).
func (h *handler) setAssistantSettings(r *http.Request, p reqctx.Principal) (int, any, error) {
	var q struct {
		AutoEnvio *bool `json:"autoEnvio"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.AutoEnvio == nil {
		return 0, nil, shared.Validation("auto_envio_requerido", "Indica si el asistente responde solo.").WithField("autoEnvio")
	}
	if err := h.s.Assistant.SetAutoReply(r.Context(), p.TallerID, p.Actor(), *q.AutoEnvio); err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{"autoEnvio": *q.AutoEnvio}, nil
}

// audit lista la bitácora con filtros por entidad o por fecha (HU-88, criterio 2).
func (h *handler) audit(r *http.Request, p reqctx.Principal) (int, any, error) {
	var since *time.Time
	if s := r.URL.Query().Get("desde"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			since = &t
		}
	}
	var out []audit.FullRecord
	err := h.s.M.Tx.Do(r.Context(), func(ctx context.Context) error {
		var err error
		out, err = h.s.M.Audit.List(ctx, p.TallerID, audit.Filter{Entity: r.URL.Query().Get("entidad"),
			UserID: r.URL.Query().Get("usuario"), Since: since, Limit: 200})
		return err
	})
	rows := []map[string]any{}
	for _, x := range out {
		rows = append(rows, map[string]any{"id": x.ID, "accion": x.Action, "entidad": x.Entity, "entidadId": x.EntityID,
			"actorTipo": x.ActorType, "actorId": x.ActorID, "antes": x.Before, "despues": x.After, "ocurridoEn": x.OccurredAt})
	}
	return http.StatusOK, map[string]any{"eventos": rows}, err
}

// extra implementa analytics.Extra.
type extra struct{ s *Services }

func (e extra) ResponseMinutes(ctx context.Context, tallerID shared.ID, since time.Time) ([]float64, error) {
	return e.s.Messaging.ResponseMinutes(ctx, tallerID, since)
}

func (e extra) FailedNotices(ctx context.Context, tallerID shared.ID, since time.Time) ([]analytics.FailedNotice, error) {
	runs, err := e.s.Automations.Runs(ctx, tallerID)
	var out []analytics.FailedNotice
	for _, r := range runs {
		if r.Result != "fallido" || r.CreatedAt.Before(since) {
			continue
		}
		name := r.Code
		if d, ferr := automationsdomain.Find(r.Code); ferr == nil {
			name = d.Name
		}
		f := analytics.FailedNotice{Name: name, OrderID: r.OrderID, At: r.CreatedAt}
		if r.Detail != nil {
			f.Detail = *r.Detail
		}
		out = append(out, f)
	}
	return out, err
}

func (e extra) HumanRequests(ctx context.Context, tallerID shared.ID) ([]analytics.HumanRequest, error) {
	list, err := e.s.Messaging.Conversations(ctx, tallerID)
	var out []analytics.HumanRequest
	for _, c := range list {
		if c.WantsHuman && c.Mode == "humano" {
			out = append(out, analytics.HumanRequest{ConversationID: c.ID, Phone: c.Phone})
		}
	}
	return out, err
}

func (e extra) PendingDrafts(ctx context.Context, tallerID shared.ID) (int, error) {
	d, err := e.s.Assistant.Drafts(ctx, tallerID, true)
	return len(d), err
}

func (e extra) LowStock(ctx context.Context, tallerID shared.ID) ([]inventory.Part, error) {
	low, err := e.s.M.Inventory.Low(ctx, tallerID)
	out := make([]inventory.Part, 0, len(low))
	for _, p := range low {
		out = append(out, inventory.Part{ID: p.ID, Ref: p.Ref, Name: p.Name, Price: p.Price, Stock: p.Stock, Active: p.Active})
	}
	return out, err
}

func (e extra) CustomerNames(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]string, error) {
	m, err := e.s.M.CustDir.SummariesByIDs(ctx, tallerID, ids)
	out := map[shared.ID]string{}
	for k, v := range m {
		out[k] = v.Name
	}
	return out, err
}
