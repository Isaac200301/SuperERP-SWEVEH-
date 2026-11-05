package app

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/customers/domain"
	"sweveh/internal/shared"
)

var ErrAnonymized = shared.Rule("cliente_anonimizado", "Los datos de este cliente ya se borraron.")

// CustomerEraser borra lo que identifica al cliente en los otros módulos (placas, conversaciones, borradores, quién
// recogió) y conserva órdenes y montos. Devuelve cuántos registros tocó en cada uno. Lo implementa la raíz de
// composición con las fachadas.
type CustomerEraser interface {
	AnonymizeCustomer(ctx context.Context, tallerID shared.ID, c domain.Customer) (map[string]int, error)
}

// SetEraser conecta el borrado en los otros módulos (composition root).
func (s *Service) SetEraser(e CustomerEraser) { s.eraser = e }

// ConsentEntry es una autorización (o rechazo) de la historia del cliente.
type ConsentEntry struct {
	State, Text, Version, Channel string
	At                            time.Time
}

// PersonalData es lo que el módulo de clientes sabe del titular, para la copia de sus datos (HU-87).
type PersonalData struct {
	Customer domain.Customer
	Consents []ConsentEntry
}

// PersonalData lee los datos del cliente y su historia de autorizaciones.
func (s *Service) PersonalData(ctx context.Context, tallerID, id shared.ID) (PersonalData, error) {
	var out PersonalData
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.ByID(ctx, tallerID, id)
		if err != nil {
			return err
		}
		out.Customer = c
		out.Consents, err = s.extra.Consents(ctx, tallerID, id)
		return err
	})
	return out, err
}

// Anonymize atiende la solicitud de borrado del titular (HU-87, Ley 1581): en una transacción borra lo que lo
// identifica aquí y en los otros módulos, y conserva sus órdenes y montos sin identificarlo. La bitácora registra quién
// lo hizo y cuánto se tocó, sin datos del titular. No se puede deshacer.
func (s *Service) Anonymize(ctx context.Context, tallerID shared.ID, actor shared.Actor, id shared.ID) (map[string]int, error) {
	var out map[string]int
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		c, err := s.repo.ByID(ctx, tallerID, id)
		if err != nil {
			return err
		}
		done, err := s.extra.Anonymized(ctx, tallerID, id)
		if err != nil {
			return err
		}
		if done {
			return ErrAnonymized
		}
		if out, err = s.eraser.AnonymizeCustomer(ctx, tallerID, c); err != nil {
			return err
		}
		now := s.clock.Now()
		if err := s.extra.Anonymize(ctx, tallerID, id, now); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cliente.anonimizar", Entity: "cliente",
			EntityID: id, After: map[string]any{"anonimizados": out}, OccurredAt: now})
	})
	return out, err
}
