package app

import (
	"bytes"
	"context"
	"errors"
	"io"

	"sweveh/internal/audit"
	"sweveh/internal/catalog/domain"
	"sweveh/internal/shared"
)

// Lista de precios por foto (HU-93, ADR-0032).

// PriceListReader lee una foto de la lista de precios y devuelve sus filas (puerto; hoy, un modelo de visión por
// OpenRouter). No decide nada: lo que lee queda como propuesta para el dueño.
type PriceListReader interface {
	Read(ctx context.Context, image []byte, mime string) ([]domain.Row, error)
}

// PhotoStore guarda la foto (el almacenamiento de la plataforma).
type PhotoStore interface {
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// ProposalRepository guarda las propuestas.
type ProposalRepository interface {
	InsertProposals(ctx context.Context, ps []domain.Proposal) error
	Proposals(ctx context.Context, tallerID shared.ID, onlyPending bool) ([]domain.Proposal, error)
	Proposal(ctx context.Context, tallerID, id shared.ID) (domain.Proposal, error)
	ResolveProposal(ctx context.Context, tallerID, id shared.ID, state string, serviceID *shared.ID, by shared.ID) (bool, error)
}

// MaxPhotoBytes es el tamaño máximo de la foto de la lista.
const MaxPhotoBytes = 8 << 20

var (
	ErrNoReader  = shared.Rule("lista_sin_ia", "Leer la lista con una foto necesita la IA configurada. Por ahora agrega los servicios a mano.")
	ErrReading   = shared.Rule("lista_no_leida", "No pudimos leer la foto en este momento. Intenta de nuevo en un minuto.")
	ErrPhotoType = shared.Validation("foto_tipo", "La foto debe ser JPG, PNG o WebP.").WithField("foto")
	ErrPhotoSize = shared.Validation("foto_tamano", "La foto pesa más de 8 MB.").WithField("foto")
	ErrTooLarge  = errors.New("catalog: foto demasiado grande")
)

// SetPriceList conecta el almacenamiento de la foto y el repositorio de propuestas (composition root).
func (s *Service) SetPriceList(photos PhotoStore, repo ProposalRepository) {
	s.photos, s.proposals = photos, repo
}

// SetPriceReader conecta la lectura de la foto (depende de la IA configurada). Sin lector, subir la foto responde que
// hace falta la IA.
func (s *Service) SetPriceReader(r PriceListReader) { s.reader = r }

// ImportPhoto lee la foto con la IA (fuera de la transacción), la guarda y deja una propuesta por fila (HU-93).
func (s *Service) ImportPhoto(ctx context.Context, actor shared.Actor, tallerID shared.ID, r io.Reader) ([]domain.Proposal, error) {
	if s.reader == nil || s.proposals == nil {
		return nil, ErrNoReader
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxPhotoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxPhotoBytes {
		return nil, ErrPhotoSize
	}
	mime := sniffImage(raw)
	if mime == "" {
		return nil, ErrPhotoType
	}
	rows, err := s.reader.Read(ctx, raw, mime)
	if err != nil {
		return nil, ErrReading
	}
	now := s.clock.Now()
	batch := shared.NewID()
	key := tallerID.String() + "/" + batch.String()
	var out []domain.Proposal
	for _, row := range rows {
		if len(out) == domain.MaxProposals {
			break
		}
		if row, ok := row.Clean(); ok {
			out = append(out, domain.Proposal{ID: shared.NewID(), TallerID: tallerID, BatchID: batch, PhotoKey: key, PhotoType: mime,
				Name: row.Name, Price: row.Price, Kind: row.Kind, State: "pendiente", CreatedAt: now})
		}
	}
	if len(out) == 0 {
		return nil, domain.ErrNoRows
	}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		if err := s.photos.Put(ctx, key, bytes.NewReader(raw), MaxPhotoBytes); err != nil {
			if errors.Is(err, ErrTooLarge) {
				return ErrPhotoSize
			}
			return err
		}
		if err := s.proposals.InsertProposals(ctx, out); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "servicio.lista_foto", Entity: "taller",
			EntityID: tallerID, After: map[string]any{"lote": batch.String(), "filas": len(out)}, OccurredAt: now})
	})
	return out, err
}

// Proposals lista las filas por revisar (o todas).
func (s *Service) Proposals(ctx context.Context, tallerID shared.ID, onlyPending bool) ([]domain.Proposal, error) {
	if s.proposals == nil {
		return []domain.Proposal{}, nil
	}
	var out []domain.Proposal
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.proposals.Proposals(ctx, tallerID, onlyPending)
		return err
	})
	return out, err
}

// ResolveProposal aprueba (con las correcciones del dueño) o descarta una fila. Aprobar crea el servicio con las mismas
// validaciones de siempre (HU-04).
func (s *Service) ResolveProposal(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, approve bool, fix domain.Row) (domain.Proposal, error) {
	if s.proposals == nil {
		return domain.Proposal{}, domain.ErrProposalNotFound
	}
	var out domain.Proposal
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		p, err := s.proposals.Proposal(ctx, tallerID, id)
		if err != nil {
			return err
		}
		if p.State != "pendiente" {
			return domain.ErrProposalResolved
		}
		state, serviceID := "descartada", (*shared.ID)(nil)
		if approve {
			in := domain.Service{TallerID: tallerID, Name: p.Name, Kind: p.Kind, Price: p.Price}
			if fix.Name != "" {
				in.Name = fix.Name
			}
			if fix.Kind != "" {
				in.Kind = fix.Kind
			}
			if fix.Price != nil {
				in.Price = fix.Price
			}
			svc, err := s.Create(ctx, actor, in)
			if err != nil {
				return err
			}
			state, serviceID = "aprobada", &svc.ID
			p.Name, p.Kind, p.Price = svc.Name, svc.Kind, svc.Price
		}
		ok, err := s.proposals.ResolveProposal(ctx, tallerID, id, state, serviceID, actor.ID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.ErrProposalResolved
		}
		p.State, p.ServiceID = state, serviceID
		out = p
		return nil
	})
	return out, err
}

// OpenProposalPhoto abre la foto de la lista de esa fila.
func (s *Service) OpenProposalPhoto(ctx context.Context, tallerID, id shared.ID) (io.ReadCloser, string, error) {
	if s.proposals == nil {
		return nil, "", domain.ErrProposalNotFound
	}
	var p domain.Proposal
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		p, err = s.proposals.Proposal(ctx, tallerID, id)
		return err
	}); err != nil {
		return nil, "", err
	}
	rc, err := s.photos.Open(ctx, p.PhotoKey)
	return rc, p.PhotoType, err
}

func sniffImage(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}
