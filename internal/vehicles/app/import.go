package app

import (
	"context"
	"errors"

	"sweveh/internal/audit"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/domain"
)

// ErrImportHasErrors: con alguna fila en error no se importa nada; se corrige y se vuelve a subir (HU-92, como HU-37).
var ErrImportHasErrors = shared.Rule("importacion_con_errores", "Hay filas con errores. Corrígelas en el archivo y súbelo de nuevo: no se importó nada.")

// CustomerRegistrar busca el cliente por celular o lo crea con la autorización pendiente (HU-95). Nunca cambia el
// nombre de un cliente que ya existe. Lo implementa la fachada de customers.
type CustomerRegistrar interface {
	FindOrCreate(ctx context.Context, tallerID shared.ID, phone, name string, actor shared.Actor) (id shared.ID, created bool, err error)
}

// SetCustomerRegistrar conecta el alta de clientes para la importación (composition root).
func (s *Service) SetCustomerRegistrar(r CustomerRegistrar) { s.registrar = r }

// ImportResult es la vista previa (o el resultado) fila por fila.
type ImportResult struct {
	Rows         []domain.ImportRow
	Warnings     []string
	Imported     int // vehículos creados
	NewCustomers int // clientes creados (los demás ya existían por celular)
}

// Import valida la plantilla de clientes y vehículos (HU-92) y, si confirm y no hay errores, crea todo en una
// transacción: el cliente se busca por celular o se crea con autorización pendiente, y el vehículo entra con motor
// pendiente si faltan esos datos. Una placa que ya existe en el taller es error: no se cambia de dueño a nadie.
func (s *Service) Import(ctx context.Context, actor shared.Actor, tallerID shared.ID, data []byte, confirm bool) (ImportResult, error) {
	file, err := domain.ParseImport(data, s.clock.Now())
	if err != nil {
		return ImportResult{}, err
	}
	res := ImportResult{Rows: file.Rows, Warnings: file.Warnings}
	err = s.uow.Do(ctx, func(ctx context.Context) error {
		bad := 0
		for i := range res.Rows {
			r := &res.Rows[i]
			if p := r.Input.Plate.String(); p != "" {
				_, err := s.repo.ByPlate(ctx, tallerID, p)
				switch {
				case err == nil:
					r.Errors = append(r.Errors, "la placa "+p+" ya está registrada en el taller")
				case !errors.Is(err, domain.ErrVehicleNotFound):
					return err
				}
			}
			if len(r.Errors) > 0 {
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
			cid, created, err := s.registrar.FindOrCreate(ctx, tallerID, r.Phone, r.Name, actor)
			if err != nil {
				return err
			}
			if created {
				res.NewCustomers++
			}
			in := r.Input
			reg, err := s.Register(ctx, RegisterVehicle{TallerID: tallerID, Actor: actor, Plate: in.Plate.String(), Kind: in.Kind,
				ConfirmSpecial: true, CustomerID: cid, Details: in.Details, Engine: in.Engine})
			if err != nil {
				return err
			}
			if reg.Existing { // otra persona la registró entre la vista previa y la confirmación
				return ErrImportHasErrors
			}
			res.Imported++
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "vehiculo.importar", Entity: "taller",
			EntityID: tallerID, After: map[string]any{"vehiculos": res.Imported, "clientesNuevos": res.NewCustomers}, OccurredAt: s.clock.Now()})
	})
	if err != nil {
		return ImportResult{Rows: res.Rows, Warnings: res.Warnings}, err
	}
	return res, nil
}
