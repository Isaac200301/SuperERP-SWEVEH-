package http

import (
	"errors"
	"io"
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
	r.With(h.guard.Require("import:run")).Post("/vehicles/import", httpx.JSON(h.log, h.importCSV))
}

type importRowDTO struct {
	Fila       int      `json:"fila"`
	Referencia string   `json:"referencia"` // la placa
	Nombre     string   `json:"nombre"`     // el cliente
	Estado     string   `json:"estado"`     // ok | error
	Errores    []string `json:"errores"`
}

// importCSV recibe la plantilla de clientes y vehículos en multipart ("archivo"). Sin ?confirmar=1 es solo la vista
// previa (HU-92). Responde como la importación de repuestos (HU-37), con los clientes nuevos.
func (h *Handler) importCSV(r *http.Request, p reqctx.Principal) (int, any, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, domain.MaxImportBytes+64<<10)
	file, _, err := r.FormFile("archivo")
	if err != nil {
		return 0, nil, domain.ErrImportFormat
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, domain.MaxImportBytes+1))
	if err != nil {
		return 0, nil, domain.ErrImportFormat
	}
	res, err := h.svc.Import(r.Context(), p.Actor(), p.TallerID, data, r.URL.Query().Get("confirmar") == "1")
	if err != nil && !errors.Is(err, app.ErrImportHasErrors) {
		return 0, nil, err
	}
	rows := make([]importRowDTO, 0, len(res.Rows))
	bad := 0
	for _, x := range res.Rows {
		d := importRowDTO{Fila: x.Line, Referencia: x.Input.Plate.String(), Nombre: x.Name, Estado: "ok", Errores: x.Errors}
		if len(x.Errors) > 0 {
			d.Estado = "error"
			bad++
		} else {
			d.Errores = []string{}
		}
		rows = append(rows, d)
	}
	avisos := res.Warnings
	if avisos == nil {
		avisos = []string{}
	}
	body := map[string]any{"filas": rows, "conError": bad, "importadas": res.Imported, "clientesNuevos": res.NewCustomers, "avisos": avisos}
	if err != nil {
		return http.StatusUnprocessableEntity, body, nil
	}
	return http.StatusOK, body, nil
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
