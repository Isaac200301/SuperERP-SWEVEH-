package http

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"sweveh/internal/orders/app"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
	"sweveh/internal/shared"
)

func (h *Handler) flowRoutes(r chi.Router) {
	r.With(h.guard.Require("order:update_state")).Post("/orders/{id}/transition", httpx.JSON(h.log, h.transition))
	r.With(h.guard.Require("order:cancel")).Post("/orders/{id}/cancel", httpx.JSON(h.log, h.cancel))
	r.With(h.guard.Require("order:assign")).Patch("/orders/{id}/assignment", httpx.JSON(h.log, h.assign))
	r.With(h.guard.Require(permRead)).Get("/orders/{id}/checklist", httpx.JSON(h.log, h.getChecklist))
	r.With(h.guard.Require("evidence:upload")).Put("/orders/{id}/checklist", httpx.JSON(h.log, h.saveChecklist))
	r.With(h.guard.Require(permRead)).Get("/orders/{id}/evidence", httpx.JSON(h.log, h.listEvidence))
	r.With(h.guard.Require("evidence:upload")).Post("/orders/{id}/evidence", httpx.JSON(h.log, h.addEvidence))
	r.With(h.guard.Require(permRead)).Get("/orders/{id}/evidence/{eid}/file", h.photo)
	r.With(h.guard.Require("vehicle:read")).Get("/vehicles/{id}/history", httpx.JSON(h.log, h.history))
}

func orderID(r *http.Request) (shared.ID, error) {
	return httpx.PathID(r, "id", domain.ErrOrderNotFound)
}

type transitionReq struct {
	Estado       string `json:"estado"`
	Version      int    `json:"version"`
	Motivo       string `json:"motivo"`
	RecogidoPor  string `json:"recogidoPor"`
	AccesoriosOK bool   `json:"accesoriosConfirmados"`
}

func (h *Handler) transition(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	var q transitionReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	to, err := domain.ParseState(q.Estado)
	if err != nil {
		return 0, nil, err
	}
	if to == domain.StateCancelled {
		return 0, nil, domain.ErrStep
	}
	o, err := h.flow.Transition(r.Context(), app.TransitionCmd{TallerID: p.TallerID, OrderID: id, Actor: p.Actor(), To: to,
		Version: q.Version, CanOverride: h.guard.Can(p, "order:override_approval"), Reason: q.Motivo,
		PickedUpBy: q.RecogidoPor, AccessoriesOK: q.AccesoriosOK})
	if err != nil {
		return 0, nil, err
	}
	v, err := h.flow.Get(r.Context(), p.TallerID, o.ID)
	return http.StatusOK, toDTO(v), err
}

type cancelReq struct {
	Motivo  string `json:"motivo"`
	Version int    `json:"version"`
}

func (h *Handler) cancel(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	var q cancelReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if _, err := h.flow.Cancel(r.Context(), p.TallerID, id, p.Actor(), q.Motivo, q.Version); err != nil {
		return 0, nil, err
	}
	v, err := h.flow.Get(r.Context(), p.TallerID, id)
	return http.StatusOK, toDTO(v), err
}

type assignReq struct {
	MecanicoID     *string    `json:"mecanicoId"`
	FechaPrometida *time.Time `json:"fechaPrometida"`
}

func (h *Handler) assign(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	var q assignReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	var mech *shared.ID
	if q.MecanicoID != nil && *q.MecanicoID != "" {
		m, err := shared.ParseID(*q.MecanicoID)
		if err != nil {
			return 0, nil, shared.Validation("mecanico_invalido", "Ese mecánico no existe.").WithField("mecanicoId")
		}
		mech = &m
	}
	if _, err := h.flow.Assign(r.Context(), p.TallerID, id, p.Actor(), mech, q.FechaPrometida); err != nil {
		return 0, nil, err
	}
	v, err := h.flow.Get(r.Context(), p.TallerID, id)
	return http.StatusOK, toDTO(v), err
}

type checklistDTO struct {
	Tipo          string             `json:"tipo"`
	Kilometraje   *int               `json:"kilometraje"`
	Combustible   *string            `json:"combustible"`
	Accesorios    []string           `json:"accesorios"`
	Salvedades    []domain.Exception `json:"salvedades"`
	LadosConFoto  []string           `json:"ladosConFoto"`
	LadosPedidos  []string           `json:"ladosPedidos"`
	Falta         []string           `json:"falta"`
	CompletadoEn  *time.Time         `json:"completadoEn"`
	Items         []domain.ItemCheck `json:"items"`
	FotosMinimas  int                `json:"fotosMinimas"`
	KmObligatorio bool               `json:"kmObligatorio"`
}

func checklistOut(v app.ChecklistView) checklistDTO {
	pedidos := domain.CarSides
	if v.Checklist.Kind == "moto" {
		pedidos = domain.MotoSides
	}
	d := checklistDTO{Tipo: v.Checklist.Kind, Kilometraje: v.Checklist.Km, Combustible: v.Checklist.Fuel,
		Accesorios: v.Checklist.Accessories, Salvedades: v.Checklist.Exceptions, LadosConFoto: v.Sides, LadosPedidos: pedidos,
		Falta: v.Missing, CompletadoEn: v.Checklist.DoneAt, Items: v.Checklist.Items, FotosMinimas: v.MinPhotos, KmObligatorio: v.MileageRequired}
	if d.Items == nil {
		d.Items = []domain.ItemCheck{}
	}
	if d.Accesorios == nil {
		d.Accesorios = []string{}
	}
	if d.Salvedades == nil {
		d.Salvedades = []domain.Exception{}
	}
	if d.Falta == nil {
		d.Falta = []string{}
	}
	return d
}

func (h *Handler) getChecklist(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	v, err := h.flow.GetChecklist(r.Context(), p.TallerID, id)
	return http.StatusOK, checklistOut(v), err
}

type checklistReq struct {
	Kilometraje *int               `json:"kilometraje"`
	Combustible *string            `json:"combustible"`
	Accesorios  []string           `json:"accesorios"`
	Salvedades  []domain.Exception `json:"salvedades"`
	Items       []domain.ItemCheck `json:"items"`
}

func (h *Handler) saveChecklist(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	var q checklistReq
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	v, err := h.flow.SaveChecklist(r.Context(), p.TallerID, id, p.Actor(), domain.Checklist{Km: q.Kilometraje, Fuel: q.Combustible,
		Accessories: append([]string{}, q.Accesorios...), Exceptions: q.Salvedades, Items: q.Items})
	return http.StatusOK, checklistOut(v), err
}

type evidenceDTO struct {
	ID             string    `json:"id"`
	Lado           *string   `json:"lado"`
	Nota           *string   `json:"nota"`
	TieneFoto      bool      `json:"tieneFoto"`
	MostrarCliente bool      `json:"mostrarCliente"`
	Etapa          string    `json:"etapa"`
	AutorID        *string   `json:"autorId"`
	CreadoEn       time.Time `json:"creadoEn"`
	GrupoID        *string   `json:"grupoId"`
	HallazgoID     *string   `json:"hallazgoId"`
}

func evidenceOut(e ports.Evidence) evidenceDTO {
	d := evidenceDTO{ID: e.ID.String(), Lado: e.Side, Nota: e.Note, TieneFoto: e.StorageKey != nil, MostrarCliente: e.ShowClient,
		Etapa: e.Stage, CreadoEn: e.CreatedAt}
	if e.AuthorID != nil {
		s := e.AuthorID.String()
		d.AutorID = &s
	}
	d.GrupoID, d.HallazgoID = idOut(e.GroupID), idOut(e.FindingID)
	return d
}

func idOut(id *shared.ID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func (h *Handler) listEvidence(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	list, err := h.flow.ListEvidence(r.Context(), p.TallerID, id)
	out := make([]evidenceDTO, 0, len(list))
	for _, e := range list {
		out = append(out, evidenceOut(e))
	}
	return http.StatusOK, map[string]any{"evidencias": out}, err
}

// addEvidence recibe multipart/form-data: foto (opcional), lado, nota, mostrarCliente.
func (h *Handler) addEvidence(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := orderID(r)
	if err != nil {
		return 0, nil, err
	}
	// Hasta 8 fotos por nota; el total del envío cabe en el límite de la plataforma (32 MB en Cloud Run).
	r.Body = http.MaxBytesReader(nil, r.Body, 32<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return 0, nil, httpx.ErrBadJSON.WithMessage("Envía la foto como formulario (multipart).")
	}
	group := shared.NewID()
	in := app.EvidenceInput{TallerID: p.TallerID, OrderID: id, Actor: p.Actor(), GroupID: &group}
	var first ports.Evidence
	photos := 0
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, nil, httpx.ErrBadJSON
		}
		switch part.FormName() {
		case "lado", "nota", "mostrarCliente", "hallazgoId":
			b, _ := io.ReadAll(io.LimitReader(part, 4096))
			s := string(b)
			switch part.FormName() {
			case "lado":
				if s != "" {
					in.Side = &s
				}
			case "nota":
				if s != "" {
					in.Note = &s
				}
			case "mostrarCliente":
				in.ShowClient, _ = strconv.ParseBool(s)
			case "hallazgoId":
				fid, err := shared.ParseID(s)
				if err != nil {
					return 0, nil, shared.NotFound("hallazgo_no_encontrado", "No encontramos ese hallazgo en la orden.")
				}
				in.FindingID = &fid
			}
		case "foto":
			// Las fotos van al final del formulario: cada una se procesa en streaming sin cargarla en memoria.
			// Todas quedan en el mismo grupo; la nota va solo en la primera.
			if photos == app.MaxPhotosPerNote {
				return 0, nil, app.ErrTooManyPhotos
			}
			in.Photo = part
			e, err := h.flow.AddEvidence(r.Context(), in)
			if err != nil {
				return 0, nil, err
			}
			if photos == 0 {
				first = e
			}
			photos++
			in.Note = nil
		}
	}
	if photos == 0 {
		in.GroupID = nil
		if first, err = h.flow.AddEvidence(r.Context(), in); err != nil {
			return 0, nil, err
		}
	}
	return http.StatusCreated, evidenceOut(first), nil
}

func (h *Handler) photo(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := orderID(r)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	eid, err := httpx.PathID(r, "eid", app.ErrEvidence)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	rc, ct, err := h.flow.OpenPhoto(r.Context(), p.TallerID, id, eid)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	defer func() { _ = rc.Close() }()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Disposition", "inline")
	_, _ = io.Copy(w, rc)
}

func (h *Handler) mine(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.flow.Mine(r.Context(), p.TallerID, p.UserID)
	out := make([]orderDTO, 0, len(list))
	for _, v := range list {
		out = append(out, toDTO(v))
	}
	return http.StatusOK, map[string]any{"ordenes": out}, err
}

type historyItemDTO struct {
	Orden      orderDTO      `json:"orden"`
	Evidencias []evidenceDTO `json:"evidencias"`
	Cotizacion any           `json:"cotizacion"`
}

// history es la ficha del vehículo (HU-13). El mecánico ve trabajos, fotos y repuestos, sin dinero.
func (h *Handler) history(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", shared.NotFound("vehiculo_no_encontrado", "No encontramos ese vehículo en tu taller."))
	if err != nil {
		return 0, nil, err
	}
	items, err := h.flow.VehicleHistory(r.Context(), p.TallerID, id)
	if err != nil {
		return 0, nil, err
	}
	money := h.guard.Can(p, "money:read")
	out := make([]historyItemDTO, 0, len(items))
	for _, it := range items {
		d := historyItemDTO{Orden: toDTO(it.View), Evidencias: []evidenceDTO{}}
		for _, e := range it.Evidence {
			d.Evidencias = append(d.Evidencias, evidenceOut(e))
		}
		if it.Quote != nil {
			lines := []map[string]any{}
			for _, l := range it.Quote.Lines {
				m := map[string]any{"tipo": l.Kind, "descripcion": l.Description, "cantidad": l.Qty}
				if money {
					m["precioUnitario"] = l.UnitPrice
				}
				lines = append(lines, m)
			}
			q := map[string]any{"estado": it.Quote.State, "lineas": lines}
			if money {
				q["total"] = it.Quote.Total
			}
			d.Cotizacion = q
		}
		out = append(out, d)
	}
	return http.StatusOK, map[string]any{"ordenes": out}, nil
}
