package app

import (
	"context"

	"sweveh/internal/audit"
	"sweveh/internal/inventory/domain"
	"sweveh/internal/shared"
)

// ErrImportHasErrors: con alguna fila en error no se importa nada; se corrige y se vuelve a subir (HU-37).
var ErrImportHasErrors = shared.Rule("importacion_con_errores", "Hay filas con errores. Corrígelas en el archivo y súbelo de nuevo: no se importó nada.")

// ImportResult es la vista previa (o el resultado) fila por fila.
type ImportResult struct {
	Rows     []domain.ImportRow
	Warnings []string
	Imported int
}

// Import valida el archivo y, si confirm y no hay errores, crea todos los repuestos en una transacción, cada uno
// con su movimiento "importacion" (HU-37). Una referencia que ya existe es error: no se pisan existencias.
func (s *Service) Import(ctx context.Context, actor shared.Actor, tallerID shared.ID, data []byte, confirm bool) (ImportResult, error) {
	file, err := domain.ParseImport(data)
	if err != nil {
		return ImportResult{}, err
	}
	rows := file.Rows
	res := ImportResult{Rows: rows, Warnings: file.Warnings}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		refs := make([]string, 0, len(rows))
		for _, r := range rows {
			refs = append(refs, r.Part.Ref)
		}
		existing, err := s.repo.ExistingRefs(ctx, tallerID, refs)
		if err != nil {
			return err
		}
		bad := 0
		for i := range res.Rows {
			if existing[res.Rows[i].Part.Ref] {
				res.Rows[i].Errors = append(res.Rows[i].Errors, "ya tienes un repuesto con esa referencia")
			}
			if len(res.Rows[i].Errors) > 0 {
				bad++
			}
		}
		if !confirm {
			return nil
		}
		if bad > 0 {
			return ErrImportHasErrors
		}
		for _, r := range res.Rows {
			if err := s.importOne(ctx, actor, tallerID, r.Part); err != nil {
				return err
			}
		}
		res.Imported = len(res.Rows)
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "repuesto.importar", Entity: "taller",
			EntityID: tallerID, After: map[string]any{"filas": res.Imported}, OccurredAt: s.clock.Now()})
	})
	if err != nil {
		return ImportResult{Rows: res.Rows, Warnings: res.Warnings}, err
	}
	return res, nil
}

func (s *Service) importOne(ctx context.Context, actor shared.Actor, tallerID shared.ID, p domain.Part) error {
	initial := p.Stock
	p.ID, p.TallerID, p.Active, p.Version, p.Stock = shared.NewID(), tallerID, true, 1, 0
	if err := s.repo.Insert(ctx, p); err != nil {
		return err
	}
	if initial == 0 {
		return nil
	}
	_, err := s.repo.Move(ctx, Movement{TallerID: tallerID, PartID: p.ID, Qty: initial, Reason: "importacion", UserID: actorID(actor)})
	return err
}

// Movements devuelve el libro del repuesto (HU-35, HU-38).
func (s *Service) Movements(ctx context.Context, tallerID, partID shared.ID) ([]domain.Movement, error) {
	var out []domain.Movement
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := s.repo.Get(ctx, tallerID, partID); err != nil {
			return err
		}
		var err error
		out, err = s.repo.Movements(ctx, tallerID, partID, 100)
		return err
	})
	return out, err
}
