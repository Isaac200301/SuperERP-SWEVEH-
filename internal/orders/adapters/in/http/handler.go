// Package http expone los casos de uso de orders.
package http

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/orders/app"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

const (
	permCreate = "order:create"
	permRead   = "order:read"
)

var ErrCursorInvalid = shared.Validation("cursor_invalido", "La página pedida no es válida.")

type Handler struct {
	svc   *app.Service
	flow  *app.Flow
	guard *authn.Guard
	log   *slog.Logger
}

func NewHandler(svc *app.Service, flow *app.Flow, guard *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{svc: svc, flow: flow, guard: guard, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require(permCreate)).Post("/orders", h.open)
	r.With(h.guard.Require(permRead)).Get("/orders", h.recent)
	r.With(h.guard.Require(permRead)).Get("/orders/mine", httpx.JSON(h.log, h.mine))
	r.With(h.guard.Require(permRead)).Get("/orders/{id}", h.get)
	h.flowRoutes(r)
}

type vehicleDTO struct {
	ID             string  `json:"id"`
	Placa          string  `json:"placa"`
	Tipo           string  `json:"tipo"`
	Marca          *string `json:"marca"`
	Linea          *string `json:"linea"`
	ModeloAnio     *int    `json:"modeloAnio"`
	MotorPendiente bool    `json:"motorPendiente"`
}

type customerDTO struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

type historyDTO struct {
	Desde      *string   `json:"desde"`
	Hacia      string    `json:"hacia"`
	UsuarioID  *string   `json:"usuarioId"`
	OcurridoEn time.Time `json:"ocurridoEn"`
}

type orderDTO struct {
	ID        string       `json:"id"`
	Numero    int          `json:"numero"`
	Codigo    string       `json:"codigo"`
	Placa     string       `json:"placa"`
	Estado    string       `json:"estado"`
	Sintoma   string       `json:"sintoma"`
	Version   int          `json:"version"`
	CreadoEn  time.Time    `json:"creadoEn"`
	Vehiculo  *vehicleDTO  `json:"vehiculo,omitempty"`
	Cliente   *customerDTO `json:"cliente,omitempty"`
	Historial []historyDTO `json:"historial,omitempty"`

	MecanicoID     *string    `json:"mecanicoId"`
	FechaPrometida *time.Time `json:"fechaPrometida"`
	Ejemplo        bool       `json:"ejemplo"`
	RecogidoPor    string     `json:"recogidoPor,omitempty"`
	MotivoCancel   string     `json:"motivoCancelacion,omitempty"`
	CerradaEn      *time.Time `json:"cerradaEn"`
	// Respuesta del cliente a la constancia de recibo (HU-19): conforme u observado.
	Constancia   *string    `json:"constancia"`
	ConstanciaEn *time.Time `json:"constanciaEn"`
}

func toDTO(v app.OrderView) orderDTO {
	o := v.Order
	d := orderDTO{
		ID: o.ID.String(), Numero: o.Number, Codigo: o.Code(), Placa: o.Plate, Estado: string(o.State),
		Sintoma: o.Symptom, Version: o.Version, CreadoEn: o.CreatedAt, FechaPrometida: o.PromisedAt, Ejemplo: o.Example,
		RecogidoPor: o.PickedUpBy, MotivoCancel: o.CancelReason, CerradaEn: o.ClosedAt, ConstanciaEn: o.ReceiptAnsweredAt,
	}
	if o.ReceiptAnswer != "" {
		d.Constancia = &o.ReceiptAnswer
	}
	if o.MechanicID != nil {
		s := o.MechanicID.String()
		d.MecanicoID = &s
	}
	if v.Vehicle != nil {
		d.Vehiculo = &vehicleDTO{
			ID: v.Vehicle.ID.String(), Placa: v.Vehicle.Plate, Tipo: v.Vehicle.Kind, Marca: v.Vehicle.Brand,
			Linea: v.Vehicle.Line, ModeloAnio: v.Vehicle.Year, MotorPendiente: v.Vehicle.EnginePending,
		}
	}
	if v.Customer != nil {
		d.Cliente = &customerDTO{ID: v.Customer.ID.String(), Nombre: v.Customer.Name}
	}
	for _, e := range v.History {
		hd := historyDTO{Hacia: string(e.To), OcurridoEn: e.OccurredAt}
		if e.From != nil {
			s := string(*e.From)
			hd.Desde = &s
		}
		if e.UserID != nil {
			s := e.UserID.String()
			hd.UsuarioID = &s
		}
		d.Historial = append(d.Historial, hd)
	}
	return d
}

type openReq struct {
	VehiculoID         string `json:"vehiculoId"`
	Placa              string `json:"placa"`
	Sintoma            string `json:"sintoma"`
	EstadoInicial      string `json:"estadoInicial"`
	ConfirmarDuplicada bool   `json:"confirmarDuplicada"`
	Ejemplo            bool   `json:"ejemplo"`
}

func (h *Handler) open(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	var req openReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	cmd := app.OpenOrder{
		TallerID: p.TallerID, Actor: p.Actor(), Plate: req.Placa, Symptom: req.Sintoma,
		InitialState: domain.State(req.EstadoInicial), ConfirmDuplicate: req.ConfirmarDuplicada, Example: req.Ejemplo,
	}
	if req.VehiculoID != "" {
		id, err := shared.ParseID(req.VehiculoID)
		if err != nil {
			httpx.WriteError(w, r, h.log, shared.NotFound("vehiculo_no_encontrado", "No encontramos ese vehículo en tu taller."))
			return
		}
		cmd.VehicleID = id
	}
	o, err := h.svc.Open(r.Context(), cmd)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	w.Header().Set("ETag", httpx.ETag(o.Version))
	w.Header().Set("Location", "/api/v1/orders/"+o.ID.String())
	httpx.WriteJSON(w, http.StatusCreated, toDTO(app.OrderView{Order: o}))
}

type pageDTO struct {
	Ordenes   []orderDTO `json:"ordenes"`
	Siguiente *string    `json:"siguiente"`
}

func (h *Handler) recent(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limite"))
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	cursor, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	f := ports.ListFilter{TallerID: p.TallerID, Limit: limit, Cursor: cursor, OnlyOpen: r.URL.Query().Get("abiertas") == "1"}
	if e := r.URL.Query().Get("estado"); e != "" {
		st, err := domain.ParseState(e)
		if err != nil {
			httpx.WriteError(w, r, h.log, err)
			return
		}
		f.State = &st
	}
	list, err := h.flow.List(r.Context(), f)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	page := pageDTO{Ordenes: make([]orderDTO, 0, len(list))}
	for _, v := range list {
		page.Ordenes = append(page.Ordenes, toDTO(v))
	}
	if len(list) == limit {
		last := list[len(list)-1].Order
		next := encodeCursor(ports.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
		page.Siguiente = &next
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.log, domain.ErrOrderNotFound)
		return
	}
	v, err := h.flow.Get(r.Context(), p.TallerID, id)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	w.Header().Set("ETag", httpx.ETag(v.Order.Version))
	httpx.WriteJSON(w, http.StatusOK, toDTO(v))
}

// El cursor es opaco para el cliente: base64 de "unixnano|uuid".
func encodeCursor(c ports.Cursor) string {
	raw := strconv.FormatInt(c.CreatedAt.UnixNano(), 10) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*ports.Cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrCursorInvalid
	}
	ts, idStr, ok := strings.Cut(string(b), "|")
	if !ok {
		return nil, ErrCursorInvalid
	}
	n, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, ErrCursorInvalid
	}
	id, err := shared.ParseID(idStr)
	if err != nil {
		return nil, ErrCursorInvalid
	}
	return &ports.Cursor{CreatedAt: time.Unix(0, n).UTC(), ID: id}, nil
}
