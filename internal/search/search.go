// Package search es la búsqueda global del panel y del piso (HU-14). Solo usa fachadas de otros módulos.
package search

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/customers"
	"sweveh/internal/orders"
	"sweveh/internal/platform/authn"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles"
)

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Handler struct {
	uow       UnitOfWork
	vehicles  vehicles.Directory
	customers customers.Directory
	orders    orders.Orders
	guard     *authn.Guard
	log       *slog.Logger
}

func NewHandler(u UnitOfWork, v vehicles.Directory, c customers.Directory, o orders.Orders, g *authn.Guard, log *slog.Logger) *Handler {
	return &Handler{uow: u, vehicles: v, customers: c, orders: o, guard: g, log: log}
}

func (h *Handler) Routes(r chi.Router) {
	r.With(h.guard.Require("vehicle:read")).Get("/search", httpx.JSON(h.log, h.search))
}

type vehicleHit struct {
	ID          string  `json:"id"`
	Placa       string  `json:"placa"`
	Tipo        string  `json:"tipo"`
	Descripcion string  `json:"descripcion"`
	ClienteID   string  `json:"clienteId"`
	Cliente     string  `json:"cliente"`
	OrdenID     *string `json:"ordenAbiertaId"`
	Orden       *string `json:"ordenAbierta"`
	Estado      *string `json:"estadoOrden"`
}

type customerHit struct {
	ID        string       `json:"id"`
	Nombre    string       `json:"nombre"`
	Celular   string       `json:"celular"`
	Vehiculos []vehicleHit `json:"vehiculos"`
}

func desc(v vehicles.Snapshot) string {
	s := ""
	for _, p := range []*string{v.Brand, v.Line} {
		if p != nil {
			s += *p + " "
		}
	}
	if v.Year != nil {
		s += itoa(*v.Year)
	}
	return s
}

func itoa(n int) string {
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// search devuelve placas que coinciden (con cliente y orden abierta) y clientes por celular o nombre con sus vehículos.
func (h *Handler) search(r *http.Request, p reqctx.Principal) (int, any, error) {
	q := r.URL.Query().Get("q")
	canSeeCustomers := h.guard.Can(p, "customer:read_all")
	out := map[string]any{"vehiculos": []vehicleHit{}, "clientes": []customerHit{}}
	err := h.uow.Do(r.Context(), func(ctx context.Context) error {
		vs, err := h.vehicles.SearchPlate(ctx, p.TallerID, q)
		if err != nil {
			return err
		}
		vh, err := h.hits(ctx, p.TallerID, vs)
		if err != nil {
			return err
		}
		out["vehiculos"] = vh
		if !canSeeCustomers {
			return nil
		}
		cs, err := h.customers.Search(ctx, p.TallerID, q)
		if err != nil {
			return err
		}
		ch := make([]customerHit, 0, len(cs))
		for _, c := range cs {
			cv, err := h.vehicles.ByCustomer(ctx, p.TallerID, c.ID)
			if err != nil {
				return err
			}
			hits, err := h.hits(ctx, p.TallerID, cv)
			if err != nil {
				return err
			}
			ch = append(ch, customerHit{ID: c.ID.String(), Nombre: c.Name, Celular: shared.NationalNumber(c.Phone), Vehiculos: hits})
		}
		out["clientes"] = ch
		return nil
	})
	return http.StatusOK, out, err
}

func (h *Handler) hits(ctx context.Context, tallerID shared.ID, vs []vehicles.Snapshot) ([]vehicleHit, error) {
	ids := make([]shared.ID, 0, len(vs))
	for _, v := range vs {
		ids = append(ids, v.CustomerID)
	}
	names, err := h.customers.SummariesByIDs(ctx, tallerID, ids)
	if err != nil {
		return nil, err
	}
	out := make([]vehicleHit, 0, len(vs))
	for _, v := range vs {
		hit := vehicleHit{ID: v.ID.String(), Placa: v.Plate, Tipo: v.Kind, Descripcion: desc(v), ClienteID: v.CustomerID.String(),
			Cliente: names[v.CustomerID].Name}
		open, err := h.orders.OpenByVehicle(ctx, tallerID, v.ID)
		if err != nil {
			return nil, err
		}
		if len(open) > 0 {
			id, code, st := open[0].ID.String(), open[0].Code, open[0].State
			hit.OrdenID, hit.Orden, hit.Estado = &id, &code, &st
		}
		out = append(out, hit)
	}
	return out, nil
}
