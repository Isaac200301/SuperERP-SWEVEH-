package postgres

import (
	"context"
	"strings"
	"time"

	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
	"sweveh/internal/vehicles/domain"
)

// Extra implementa app.Extra con pgx (ADR-0024).
type Extra struct{}

type vRow struct {
	ID           shared.ID `db:"id"`
	TallerID     shared.ID `db:"taller_id"`
	ClienteID    shared.ID `db:"cliente_id"`
	Tipo         string    `db:"tipo"`
	Placa        string    `db:"placa"`
	FormatoPlaca string    `db:"formato_placa"`
	Marca        *string   `db:"marca"`
	Linea        *string   `db:"linea"`
	ModeloAnio   *int      `db:"modelo_anio"`
	Kilometraje  *int      `db:"kilometraje"`
	CilindrajeCc *int      `db:"cilindraje_cc"`
	Turbo        *bool     `db:"turbo"`
	Energia      *string   `db:"energia"`
	Aspiracion   *string   `db:"aspiracion"`
	Transmision  *string   `db:"transmision"`
	Traccion     *string   `db:"traccion"`
	Ciclo        *string   `db:"ciclo"`
	Refrig       *string   `db:"refrigeracion"`
	Color        *string   `db:"color"`
	Carroceria   *string   `db:"carroceria"`
	Llanta       *string   `db:"llanta"`
	Version      int       `db:"version"`
	CreatedAt    time.Time `db:"created_at"`
}

const vCols = `id, taller_id, cliente_id, tipo, placa, formato_placa, marca, linea, modelo_anio, kilometraje, cilindraje_cc, turbo,
	energia, aspiracion, transmision, traccion, ciclo, refrigeracion, color, carroceria, llanta, version, created_at`

func (r vRow) domain() domain.Vehicle {
	return domain.Vehicle{ID: r.ID, TallerID: r.TallerID, CustomerID: r.ClienteID, Kind: domain.Kind(r.Tipo),
		Plate:   domain.RestorePlate(r.Placa, domain.PlateFormat(r.FormatoPlaca)),
		Details: domain.Details{Brand: r.Marca, Line: r.Linea, Year: r.ModeloAnio, Mileage: r.Kilometraje, Color: r.Color, Body: r.Carroceria, Tire: r.Llanta},
		Engine: domain.Engine{DisplacementCC: r.CilindrajeCc, Turbo: r.Turbo, Energy: r.Energia, Aspiration: r.Aspiracion,
			Transmission: r.Transmision, Drive: r.Traccion, Cycle: r.Ciclo, Cooling: r.Refrig},
		Version: r.Version, CreatedAt: r.CreatedAt}
}

func vList(rows []vRow, err error) ([]domain.Vehicle, error) {
	out := make([]domain.Vehicle, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

func (Extra) SearchPlate(ctx context.Context, tallerID shared.ID, q string, limit int) ([]domain.Vehicle, error) {
	q = strings.NewReplacer(`\`, ``, `%`, ``, `_`, ``).Replace(q)
	if q == "" {
		return nil, nil
	}
	return vList(db.Many[vRow](ctx, `SELECT `+vCols+` FROM vehicles.vehiculo WHERE taller_id = $1 AND placa LIKE '%' || $2 || '%'
		ORDER BY placa LIMIT $3`, tallerID, q, limit))
}

func (Extra) ByCustomer(ctx context.Context, tallerID, customerID shared.ID) ([]domain.Vehicle, error) {
	return vList(db.Many[vRow](ctx, `SELECT `+vCols+` FROM vehicles.vehiculo WHERE taller_id = $1 AND cliente_id = $2 ORDER BY created_at`,
		tallerID, customerID))
}

func (Extra) ChangeOwner(ctx context.Context, v domain.Vehicle, prev, next shared.ID, by *shared.ID, at time.Time) error {
	n, err := db.Exec(ctx, `UPDATE vehicles.vehiculo SET cliente_id = $3, version = $4, updated_at = $5
		WHERE taller_id = $1 AND id = $2 AND version = $6`, v.TallerID, v.ID, next, v.Version, at, v.Version-1)
	if err != nil {
		return err
	}
	if n == 0 {
		return shared.ErrVersionConflict
	}
	_, err = db.Exec(ctx, `INSERT INTO vehicles.propietario_historial (id, taller_id, vehiculo_id, cliente_anterior, cliente_nuevo, cambiado_en, usuario_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, shared.NewID(), v.TallerID, v.ID, prev, next, at, by)
	return err
}
