package postgres

import (
	"context"
	"time"

	"sweveh/internal/identity/app"
	"sweveh/internal/identity/ports"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Members implementa app.Members con pgx.
type Members struct{}

type memberRow struct {
	UserID    shared.ID `db:"usuario_id"`
	Nombre    string    `db:"nombre"`
	Celular   string    `db:"celular"`
	Rol       string    `db:"rol"`
	Activo    bool      `db:"activo"`
	CreatedAt time.Time `db:"created_at"`
}

func (r memberRow) member() app.Member {
	return app.Member{UserID: r.UserID, Name: r.Nombre, Phone: r.Celular, Role: r.Rol, Active: r.Activo, CreatedAt: r.CreatedAt}
}

const memberSQL = `SELECT m.usuario_id, u.nombre, u.celular, m.rol, m.activo, m.created_at
	FROM identity.membresia m JOIN identity.usuario u ON u.id = m.usuario_id WHERE m.taller_id = $1`

func (Members) List(ctx context.Context, tallerID shared.ID) ([]app.Member, error) {
	rows, err := db.Many[memberRow](ctx, memberSQL+` ORDER BY m.created_at`, tallerID)
	out := make([]app.Member, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.member())
	}
	return out, err
}

func (Members) Get(ctx context.Context, tallerID, userID shared.ID) (app.Member, error) {
	r, err := db.One[memberRow](ctx, app.ErrMemberNF, memberSQL+` AND m.usuario_id = $2`, tallerID, userID)
	return r.member(), err
}

func (Members) Upsert(ctx context.Context, tallerID, userID shared.ID, role string, at time.Time) error {
	_, err := db.Exec(ctx, `INSERT INTO identity.membresia (taller_id, usuario_id, rol, created_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (taller_id, usuario_id) DO UPDATE SET rol = EXCLUDED.rol, activo = true`, tallerID, userID, role, at)
	return err
}

func (Members) SetActive(ctx context.Context, tallerID, userID shared.ID, active bool) error {
	_, err := db.Exec(ctx, `UPDATE identity.membresia SET activo = $3 WHERE taller_id = $1 AND usuario_id = $2`, tallerID, userID, active)
	return err
}

func (Members) RevokeSessions(ctx context.Context, tallerID, userID shared.ID, at time.Time, reason string) error {
	_, err := db.Exec(ctx, `UPDATE identity.sesion SET revocada_en = $3, motivo_revocacion = $4
		WHERE taller_id = $1 AND usuario_id = $2 AND revocada_en IS NULL`, tallerID, userID, at, reason)
	return err
}

// Workshops lista los talleres activos del usuario. Requiere app.usuario_id (política de membresia);
// el nombre del taller se lee con un SELECT por id, que su política solo permite al taller actual, por
// eso se devuelve solo el id y el nombre lo completa quien llama si hace falta.
func (Members) Workshops(ctx context.Context, userID shared.ID) ([]ports.Workshop, error) {
	type row struct {
		TallerID shared.ID `db:"taller_id"`
	}
	rows, err := db.Many[row](ctx, `SELECT taller_id FROM identity.membresia WHERE usuario_id = $1 AND activo ORDER BY created_at`, userID)
	out := make([]ports.Workshop, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.Workshop{ID: r.TallerID})
	}
	return out, err
}
