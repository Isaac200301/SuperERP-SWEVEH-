// Package http expone los casos de uso de vehicles.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/app"
	"sweveh/internal/vehicles/domain"
)

const (
	permCreate = "vehicle:create"
	permUpdate = "vehicle:update"
	permRead   = "vehicle:read"
)

type Handler struct {
	svc   *app.Service
	guard *authn.Guard
	log   *slog.Logger
}

func NewHandler(svc *app.Service, guard *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{svc: svc, guard: guard, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require(permCreate)).Post("/vehicles", h.register)
	r.With(h.guard.Require(permRead)).Get("/vehicles/by-plate/{placa}", h.byPlate)
	r.With(h.guard.Require(permRead)).Get("/vehicles/{id}", h.byID)
	r.With(h.guard.Require(permUpdate)).Patch("/vehicles/{id}/engine", h.updateEngine)
}

type vehicleDTO struct {
	ID             string    `json:"id"`
	ClienteID      string    `json:"clienteId"`
	Tipo           string    `json:"tipo"`
	Placa          string    `json:"placa"`
	FormatoPlaca   string    `json:"formatoPlaca"`
	Marca          *string   `json:"marca"`
	Linea          *string   `json:"linea"`
	ModeloAnio     *int      `json:"modeloAnio"`
	Kilometraje    *int      `json:"kilometraje"`
	CilindrajeCc   *int      `json:"cilindrajeCc"`
	Turbo          *bool     `json:"turbo"`
	ficha
	MotorPendiente bool      `json:"motorPendiente"`
	Version        int       `json:"version"`
	CreadoEn       time.Time `json:"creadoEn"`
}

func toDTO(v domain.Vehicle) vehicleDTO {
	return vehicleDTO{
		ID: v.ID.String(), ClienteID: v.CustomerID.String(), Tipo: string(v.Kind),
		Placa: v.Plate.String(), FormatoPlaca: string(v.Plate.Format()),
		Marca: v.Details.Brand, Linea: v.Details.Line, ModeloAnio: v.Details.Year, Kilometraje: v.Details.Mileage,
		CilindrajeCc: v.Engine.DisplacementCC, Turbo: v.Engine.Turbo, MotorPendiente: v.Engine.Pending(v.Kind),
		ficha: ficha{Energia: v.Engine.Energy, Aspiracion: v.Engine.Aspiration, Transmision: v.Engine.Transmission, Traccion: v.Engine.Drive,
			Ciclo: v.Engine.Cycle, Refrigeracion: v.Engine.Cooling, Color: v.Details.Color, Carroceria: v.Details.Body, Llanta: v.Details.Tire},
		Version: v.Version, CreadoEn: v.CreatedAt,
	}
}

func writeVehicle(w http.ResponseWriter, status int, v domain.Vehicle, existing *bool) {
	w.Header().Set("ETag", httpx.ETag(v.Version))
	if existing == nil {
		httpx.WriteJSON(w, status, toDTO(v))
		return
	}
	httpx.WriteJSON(w, status, registerResp{Vehiculo: toDTO(v), Existente: *existing})
}

type registerReq struct {
	Placa             string  `json:"placa"`
	Tipo              string  `json:"tipo"`
	ConfirmarEspecial bool    `json:"confirmarPlacaEspecial"`
	ClienteID         string  `json:"clienteId"`
	Marca             *string `json:"marca"`
	Linea             *string `json:"linea"`
	ModeloAnio        *int    `json:"modeloAnio"`
	Kilometraje       *int    `json:"kilometraje"`
	Cilindraje        *string `json:"cilindraje"` // "1.2 L" o "1200": se guarda en cc (HU-11)
	Turbo             *bool   `json:"turbo"`      // API anterior; si llega aspiración, manda la aspiración
	ficha
}

// ficha son los datos del vehículo que pidió el taller (decisión del 3-nov-2026). Todos opcionales.
type ficha struct {
	Energia       *string `json:"energia"`
	Aspiracion    *string `json:"aspiracion"`
	Transmision   *string `json:"transmision"`
	Traccion      *string `json:"traccion"`
	Ciclo         *string `json:"ciclo"`
	Refrigeracion *string `json:"refrigeracion"`
	Color         *string `json:"color"`
	Carroceria    *string `json:"carroceria"`
	Llanta        *string `json:"llanta"`
}

func (f ficha) details() domain.Details {
	return domain.Details{Color: f.Color, Body: f.Carroceria, Tire: f.Llanta}
}

type registerResp struct {
	Vehiculo  vehicleDTO `json:"vehiculo"`
	Existente bool       `json:"existente"`
}

func parseEngine(cil *string, turbo *bool, f ficha) (domain.Engine, error) {
	e := domain.Engine{Turbo: turbo, Energy: f.Energia, Aspiration: f.Aspiracion, Transmission: f.Transmision,
		Drive: f.Traccion, Cycle: f.Ciclo, Cooling: f.Refrigeracion}
	if cil != nil && *cil != "" {
		cc, err := domain.ParseDisplacement(*cil)
		if err != nil {
			return domain.Engine{}, err
		}
		e.DisplacementCC = &cc
	}
	return e, nil
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	var req registerReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	engine, err := parseEngine(req.Cilindraje, req.Turbo, req.ficha)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	customerID := shared.NilID
	if req.ClienteID != "" {
		if customerID, err = shared.ParseID(req.ClienteID); err != nil {
			httpx.WriteError(w, r, h.log, app.ErrCustomerRequired)
			return
		}
	}
	res, err := h.svc.Register(r.Context(), app.RegisterVehicle{
		TallerID: p.TallerID, Actor: p.Actor(), Plate: req.Placa, Kind: req.Tipo,
		ConfirmSpecial: req.ConfirmarEspecial, CustomerID: customerID,
		Details: domain.Details{Brand: req.Marca, Line: req.Linea, Year: req.ModeloAnio, Mileage: req.Kilometraje,
			Color: req.Color, Body: req.Carroceria, Tire: req.Llanta},
		Engine:  engine,
	})
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	status := http.StatusCreated
	if res.Existing {
		status = http.StatusOK
	}
	writeVehicle(w, status, res.Vehicle, &res.Existing)
}

func (h *Handler) byPlate(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	v, err := h.svc.ByPlate(r.Context(), p.TallerID, chi.URLParam(r, "placa"))
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	writeVehicle(w, http.StatusOK, v, nil)
}

func (h *Handler) byID(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.log, domain.ErrVehicleNotFound)
		return
	}
	v, err := h.svc.ByID(r.Context(), p.TallerID, id)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	writeVehicle(w, http.StatusOK, v, nil)
}

type engineReq struct {
	Cilindraje *string `json:"cilindraje"`
	Turbo      *bool   `json:"turbo"`
	ficha
}

func (h *Handler) updateEngine(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := shared.ParseID(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, h.log, domain.ErrVehicleNotFound)
		return
	}
	version, err := httpx.IfMatchVersion(r)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	var req engineReq
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	engine, err := parseEngine(req.Cilindraje, req.Turbo, req.ficha)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	v, err := h.svc.UpdateEngine(r.Context(), app.UpdateEngine{
		TallerID: p.TallerID, Actor: p.Actor(), VehicleID: id, Engine: engine, Details: req.details(), ExpectedVersion: version,
	})
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	writeVehicle(w, http.StatusOK, v, nil)
}
