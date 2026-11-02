package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/app"
	"sweveh/internal/vehicles/domain"
)

// ExtraRoutes: vehículos de un cliente y cambio de dueño (HU-12).
func (h *Handler) ExtraRoutes(r chi.Router) {
	r.With(h.guard.Require(permRead)).Get("/customers/{id}/vehicles", httpx.JSON(h.log, h.byCustomer))
	r.With(h.guard.Require("vehicle:change_owner")).Patch("/vehicles/{id}/owner", httpx.JSON(h.log, h.changeOwner))
}

func (h *Handler) byCustomer(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", shared.NotFound("cliente_no_encontrado", "No encontramos ese cliente en tu taller."))
	if err != nil {
		return 0, nil, err
	}
	list, err := h.svc.ByCustomer(r.Context(), p.TallerID, id)
	out := make([]vehicleDTO, 0, len(list))
	for _, v := range list {
		out = append(out, toDTO(v))
	}
	return http.StatusOK, map[string]any{"vehiculos": out}, err
}

func (h *Handler) changeOwner(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrVehicleNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		ClienteID string `json:"clienteId"`
		Version   int    `json:"version"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	cid, err := shared.ParseID(q.ClienteID)
	if err != nil {
		return 0, nil, app.ErrCustomerRequired
	}
	v, err := h.svc.ChangeOwner(r.Context(), p.TallerID, id, cid, p.Actor(), q.Version)
	return http.StatusOK, toDTO(v), err
}
