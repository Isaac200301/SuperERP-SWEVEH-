package app

import (
	"context"

	"sweveh/internal/audit"
	"sweveh/internal/customers/domain"
	"sweveh/internal/shared"
)

var (
	ErrMergeSame   = shared.Validation("unir_mismo_cliente", "Elige dos clientes distintos para unir.").WithField("unir")
	ErrMergeMerged = shared.Rule("cliente_ya_unido", "Ese cliente ya se unió a otro. Recarga la lista.")
)

// CustomerMover pasa al cliente que se conserva lo que otro módulo tiene del repetido (vehículos, órdenes,
// conversaciones). Devuelve cuántos registros movió. Lo implementa la raíz de composición con las fachadas.
type CustomerMover interface {
	MoveCustomer(ctx context.Context, tallerID, from, to shared.ID) (map[string]int, error)
}

// SetMover conecta lo que se mueve al unir clientes (composition root).
func (s *Service) SetMover(m CustomerMover) { s.mover = m }

// DuplicateGroup son los clientes que parecen la misma persona (mismo nombre).
type DuplicateGroup struct{ Customers []domain.Customer }

// Duplicates agrupa los posibles repetidos por nombre (HU-15, criterio 1).
func (s *Service) Duplicates(ctx context.Context, tallerID shared.ID) ([]DuplicateGroup, error) {
	var out []DuplicateGroup
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		list, err := s.extra.Duplicates(ctx, tallerID, 200)
		if err != nil {
			return err
		}
		byName := map[string]int{}
		for _, c := range list {
			k := domain.NameKey(c.Name)
			i, ok := byName[k]
			if !ok {
				i = len(out)
				byName[k] = i
				out = append(out, DuplicateGroup{})
			}
			out[i].Customers = append(out[i].Customers, c)
		}
		return nil
	})
	return out, err
}

// MergeResult cuenta lo que pasó al cliente que se conserva.
type MergeResult struct {
	Kept  domain.Customer
	Moved map[string]int
}

// Merge une el cliente repetido (drop) al que se conserva (keep) en una transacción: sus vehículos, órdenes y
// conversaciones pasan a keep y drop queda marcado, sin borrarse. Queda en la bitácora quién lo hizo y qué se unió
// (HU-15, criterio 2). El nombre, el celular y la autorización que valen son los del que se conserva.
func (s *Service) Merge(ctx context.Context, tallerID shared.ID, actor shared.Actor, keepID, dropID shared.ID) (MergeResult, error) {
	if keepID == dropID {
		return MergeResult{}, ErrMergeSame
	}
	var out MergeResult
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		keep, err := s.repo.ByID(ctx, tallerID, keepID)
		if err != nil {
			return err
		}
		drop, err := s.repo.ByID(ctx, tallerID, dropID)
		if err != nil {
			return err
		}
		for _, id := range []shared.ID{keepID, dropID} {
			merged, err := s.extra.Merged(ctx, tallerID, id)
			if err != nil {
				return err
			}
			if merged {
				return ErrMergeMerged
			}
		}
		moved, err := s.mover.MoveCustomer(ctx, tallerID, dropID, keepID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		if err := s.extra.MarkMerged(ctx, tallerID, dropID, keepID, now); err != nil {
			return err
		}
		out = MergeResult{Kept: keep, Moved: moved}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cliente.unir", Entity: "cliente", EntityID: keepID,
			Before:     map[string]any{"unido": map[string]any{"id": dropID.String(), "nombre": drop.Name, "celular": drop.Phone}},
			After:      map[string]any{"conservado": map[string]any{"id": keepID.String(), "nombre": keep.Name, "celular": keep.Phone}, "movidos": moved},
			OccurredAt: now})
	})
	return out, err
}
