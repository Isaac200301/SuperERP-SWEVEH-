package http

import (
	"io"
	"net/http"

	"sweveh/internal/catalog/app"
	"sweveh/internal/catalog/domain"
	"sweveh/internal/platform/httpx"
	"sweveh/internal/platform/reqctx"
)

// Lista de precios por foto (HU-93, ADR-0032): subir la foto, ver las filas por revisar con la foto al lado y aprobar
// o descartar cada una.

type proposalDTO struct {
	ID       string  `json:"id"`
	Lote     string  `json:"loteId"`
	Nombre   string  `json:"nombre"`
	Precio   *int64  `json:"precio"`
	Tipo     string  `json:"tipoVehiculo"`
	Estado   string  `json:"estado"`
	Servicio *string `json:"servicioId"`
}

func proposalOut(p domain.Proposal) proposalDTO {
	d := proposalDTO{ID: p.ID.String(), Lote: p.BatchID.String(), Nombre: p.Name, Precio: p.Price, Tipo: p.Kind, Estado: p.State}
	if p.ServiceID != nil {
		s := p.ServiceID.String()
		d.Servicio = &s
	}
	return d
}

func proposalsOut(list []domain.Proposal) map[string]any {
	out := make([]proposalDTO, 0, len(list))
	for _, p := range list {
		out = append(out, proposalOut(p))
	}
	return map[string]any{"propuestas": out}
}

// importPhoto recibe la foto en multipart ("foto").
func (h *Handler) importPhoto(r *http.Request, p reqctx.Principal) (int, any, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, app.MaxPhotoBytes+(64<<10))
	file, _, err := r.FormFile("foto")
	if err != nil {
		return 0, nil, httpx.ErrBadJSON.WithMessage("Envía la foto como formulario (multipart), en el campo foto.")
	}
	defer func() { _ = file.Close() }()
	list, err := h.svc.ImportPhoto(r.Context(), p.Actor(), p.TallerID, file)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusCreated, proposalsOut(list), nil
}

func (h *Handler) proposals(r *http.Request, p reqctx.Principal) (int, any, error) {
	list, err := h.svc.Proposals(r.Context(), p.TallerID, r.URL.Query().Get("todas") != "1")
	return http.StatusOK, proposalsOut(list), err
}

func (h *Handler) resolveProposal(r *http.Request, p reqctx.Principal) (int, any, error) {
	id, err := httpx.PathID(r, "id", domain.ErrProposalNotFound)
	if err != nil {
		return 0, nil, err
	}
	var q struct {
		Decision string `json:"decision"` // aprobar | descartar
		Nombre   string `json:"nombre"`
		Precio   *int64 `json:"precio"`
		Tipo     string `json:"tipoVehiculo"`
	}
	if err := httpx.Decode(r, &q); err != nil {
		return 0, nil, err
	}
	if q.Decision != "aprobar" && q.Decision != "descartar" {
		return 0, nil, httpx.ErrBadJSON.WithMessage("La decisión es aprobar o descartar.")
	}
	out, err := h.svc.ResolveProposal(r.Context(), p.Actor(), p.TallerID, id, q.Decision == "aprobar",
		domain.Row{Name: q.Nombre, Price: q.Precio, Kind: q.Tipo})
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, proposalOut(out), nil
}

func (h *Handler) proposalPhoto(w http.ResponseWriter, r *http.Request) {
	p, _ := reqctx.PrincipalFrom(r.Context())
	id, err := httpx.PathID(r, "id", domain.ErrProposalNotFound)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	rc, ct, err := h.svc.OpenProposalPhoto(r.Context(), p.TallerID, id)
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
