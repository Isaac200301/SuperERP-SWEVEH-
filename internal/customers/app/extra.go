package app

import (
	"context"
	"strings"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/customers/domain"
	"sweveh/internal/shared"
)

// Extra son lecturas y cambios agregados para búsqueda, panel y autorización de datos (HU-14, HU-85, HU-86, HU-95).
type Extra interface {
	Search(ctx context.Context, tallerID shared.ID, digits, text string, limit int) ([]domain.Customer, error)
	List(ctx context.Context, tallerID shared.ID, limit int) ([]domain.Customer, error)
	SetConsent(ctx context.Context, tallerID, id shared.ID, st domain.ConsentStatus, rec ConsentRecord) error
	SetName(ctx context.Context, tallerID, id shared.ID, name string) error
}

// ConsentRecord es la prueba de la autorización del cliente (Ley 1581, Decreto 1377 art. 8).
type ConsentRecord struct {
	Text    string
	Version string
	Channel string // whatsapp | mostrador | importacion
	Ref     string // id del mensaje si llegó por WhatsApp
	At      time.Time
	By      *shared.ID
}

func (s *Service) SetExtra(e Extra) { s.extra = e }

// Search busca por parte del celular o del nombre (HU-14).
func (s *Service) Search(ctx context.Context, tallerID shared.ID, q string) ([]domain.Customer, error) {
	q = strings.TrimSpace(q)
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, q)
	text := ""
	if len(digits) < 3 {
		digits, text = "", q
	}
	var out []domain.Customer
	if digits == "" && len([]rune(text)) < 2 {
		return out, nil
	}
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.extra.Search(ctx, tallerID, digits, text, 20)
		return err
	})
	return out, err
}

func (s *Service) List(ctx context.Context, tallerID shared.ID) ([]domain.Customer, error) {
	var out []domain.Customer
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.extra.List(ctx, tallerID, 200)
		return err
	})
	return out, err
}

// SetConsent registra la autorización (o el rechazo) del cliente con su prueba (HU-85, HU-86).
// Corre en la transacción del llamador si la hay.
func (s *Service) SetConsent(ctx context.Context, tallerID, customerID shared.ID, actor shared.Actor, granted bool, rec ConsentRecord) error {
	st := domain.ConsentRejected
	if granted {
		st = domain.ConsentGranted
	}
	return s.uow.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.ByID(ctx, tallerID, customerID)
		if err != nil {
			return err
		}
		if err := s.extra.SetConsent(ctx, tallerID, customerID, st, rec); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cliente.autorizacion", Entity: "cliente",
			EntityID: customerID, Before: map[string]any{"estado": string(c.Consent)},
			After: map[string]any{"estado": string(st), "canal": rec.Channel, "version": rec.Version}, OccurredAt: rec.At}); err != nil {
			return err
		}
		return s.events.Publish(ctx, tallerID, shared.Event{Type: "customers.autorizacion", AggregateID: customerID, OccurredAt: rec.At,
			Payload: map[string]any{"estado": string(st), "canal": rec.Channel}})
	})
}

// FillName pone el nombre que el cliente le dio al asesor (ADR-0029), solo si el que tiene es de relleno. false = no
// cambió nada (ya tenía un nombre real).
func (s *Service) FillName(ctx context.Context, tallerID, customerID shared.ID, actor shared.Actor, raw string) (bool, error) {
	name, err := domain.CleanName(raw)
	if err != nil {
		return false, err
	}
	changed := false
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.ByID(ctx, tallerID, customerID)
		if err != nil || !domain.IsPlaceholderName(c.Name) || c.Name == name {
			return err
		}
		if err := s.extra.SetName(ctx, tallerID, customerID, name); err != nil {
			return err
		}
		changed = true
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cliente.nombre", Entity: "cliente",
			EntityID: customerID, Before: map[string]any{"nombre": c.Name}, After: map[string]any{"nombre": name}, OccurredAt: s.clock.Now()})
	})
	return changed, err
}
