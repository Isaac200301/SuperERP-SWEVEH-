package app

import (
	"bytes"
	"context"
	"errors"
	"io"

	"sweveh/internal/audit"
	"sweveh/internal/inventory/domain"
	"sweveh/internal/shared"
)

// Vitrina de WhatsApp (ADR-0029): el taller marca qué repuestos o accesorios ofrece el asesor, con una foto.

// PhotoStore guarda la foto del repuesto (el almacenamiento de la plataforma, como la evidencia de las órdenes).
type PhotoStore interface {
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// MaxPhotoBytes: una foto de producto basta con 5 MB (WhatsApp acepta imágenes de hasta 5 MB).
const MaxPhotoBytes = 5 << 20

var (
	ErrPhotoType = shared.Validation("foto_tipo", "La foto debe ser JPG, PNG o WebP.").WithField("foto")
	ErrPhotoSize = shared.Validation("foto_tamano", "La foto pesa más de 5 MB.").WithField("foto")
	ErrNoPhoto   = shared.NotFound("foto_no_encontrada", "Ese repuesto no tiene foto.")
	ErrTooLarge  = errors.New("inventory: foto demasiado grande")
)

// SetPhotoStore conecta el almacenamiento de fotos (composition root).
func (s *Service) SetPhotoStore(st PhotoStore) { s.photos = st }

// Showcase lista la vitrina: activos y marcados para ofrecer por WhatsApp.
func (s *Service) Showcase(ctx context.Context, tallerID shared.ID, q string, limit int) ([]domain.Part, error) {
	var out []domain.Part
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.repo.Showcase(ctx, tallerID, q, limit)
		return err
	})
	return out, err
}

// SetPhoto guarda la foto del repuesto (reemplaza la anterior).
func (s *Service) SetPhoto(ctx context.Context, actor shared.Actor, tallerID, id shared.ID, r io.Reader) (domain.Part, error) {
	head := make([]byte, 512)
	n, _ := io.ReadFull(r, head)
	mime := sniffImage(head[:n])
	if mime == "" {
		return domain.Part{}, ErrPhotoType
	}
	var out domain.Part
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		p, err := s.repo.Get(ctx, tallerID, id)
		if err != nil {
			return err
		}
		key := tallerID.String() + "/" + shared.NewID().String()
		if err := s.photos.Put(ctx, key, io.MultiReader(bytes.NewReader(head[:n]), r), MaxPhotoBytes); err != nil {
			if errors.Is(err, ErrTooLarge) {
				return ErrPhotoSize
			}
			return err
		}
		if err := s.repo.SetPhoto(ctx, tallerID, id, key, mime); err != nil {
			return err
		}
		p.PhotoKey, p.PhotoType = key, mime
		out = p
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "repuesto.foto", Entity: "repuesto",
			EntityID: id, After: map[string]any{"foto": key}, OccurredAt: s.clock.Now()})
	})
	return out, err
}

// OpenPhoto abre la foto del repuesto con su tipo.
func (s *Service) OpenPhoto(ctx context.Context, tallerID, id shared.ID) (io.ReadCloser, string, error) {
	var p domain.Part
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		p, err = s.repo.Get(ctx, tallerID, id)
		return err
	}); err != nil {
		return nil, "", err
	}
	if p.PhotoKey == "" {
		return nil, "", ErrNoPhoto
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
