package postgres

import (
	"context"
	"time"

	"sweveh/internal/customers/app"
	"sweveh/internal/customers/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Extra implementa app.Extra con pgx (ADR-0024).
type Extra struct{}

type cRow struct {
	ID        shared.ID `db:"id"`
	TallerID  shared.ID `db:"taller_id"`
	Celular   string    `db:"celular"`
	Nombre    string    `db:"nombre"`
	Correo    *string   `db:"correo"`
	Estado    string    `db:"autorizacion_estado"`
	Version   int       `db:"version"`
	CreatedAt time.Time `db:"created_at"`
}

const cCols = `id, taller_id, celular, nombre, correo, autorizacion_estado, version, created_at`

func cList(rows []cRow, err error) ([]domain.Customer, error) {
	out := make([]domain.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Customer{ID: r.ID, TallerID: r.TallerID, Phone: r.Celular, Name: r.Nombre, Email: r.Correo,
			Consent: domain.ConsentStatus(r.Estado), Version: r.Version, CreatedAt: r.CreatedAt})
	}
	return out, err
}

func (Extra) Search(ctx context.Context, tallerID shared.ID, digits, text string, limit int) ([]domain.Customer, error) {
	if digits != "" {
		return cList(db.Many[cRow](ctx, `SELECT `+cCols+` FROM customers.cliente WHERE taller_id = $1 AND fusionado_con IS NULL AND anonimizado_en IS NULL AND celular LIKE '%' || $2 || '%'
			ORDER BY nombre LIMIT $3`, tallerID, digits, limit))
	}
	return cList(db.Many[cRow](ctx, `SELECT `+cCols+` FROM customers.cliente WHERE taller_id = $1 AND fusionado_con IS NULL AND anonimizado_en IS NULL
		AND nombre ILIKE '%' || replace(replace(replace($2, '\', '\\'), '%', '\%'), '_', '\_') || '%' ORDER BY nombre LIMIT $3`, tallerID, text, limit))
}

func (Extra) List(ctx context.Context, tallerID shared.ID, limit int) ([]domain.Customer, error) {
	return cList(db.Many[cRow](ctx, `SELECT `+cCols+` FROM customers.cliente WHERE taller_id = $1 AND fusionado_con IS NULL AND anonimizado_en IS NULL
		ORDER BY created_at DESC LIMIT $2`, tallerID, limit))
}

// Duplicates son los clientes vigentes que comparten nombre (sin tildes, mayúsculas ni espacios de más): los posibles
// repetidos de HU-15, agrupados por nombre.
func (Extra) Duplicates(ctx context.Context, tallerID shared.ID, limit int) ([]domain.Customer, error) {
	return cList(db.Many[cRow](ctx, `WITH c AS (
			SELECT `+cCols+`, lower(btrim(regexp_replace(translate(nombre, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun'), '\s+', ' ', 'g'))) AS clave
			FROM customers.cliente WHERE taller_id = $1 AND fusionado_con IS NULL AND anonimizado_en IS NULL)
		SELECT `+cCols+` FROM c WHERE clave IN (SELECT clave FROM c GROUP BY clave HAVING count(*) > 1)
		ORDER BY clave, created_at LIMIT $2`, tallerID, limit))
}

// MarkMerged deja al cliente unido al que se conserva, y también a los que ya se habían unido a él.
func (Extra) MarkMerged(ctx context.Context, tallerID, dropID, keepID shared.ID, at time.Time) error {
	_, err := db.Exec(ctx, `UPDATE customers.cliente SET fusionado_con = $3, fusionado_en = $4, version = version + 1, updated_at = now()
		WHERE taller_id = $1 AND (id = $2 OR fusionado_con = $2)`, tallerID, dropID, keepID, at)
	return err
}

// Merged dice si el cliente ya se unió a otro.
func (Extra) Merged(ctx context.Context, tallerID, id shared.ID) (bool, error) {
	type r struct {
		Merged bool `db:"unido"`
	}
	rows, err := db.Many[r](ctx, `SELECT fusionado_con IS NOT NULL AS unido FROM customers.cliente WHERE taller_id = $1 AND id = $2`, tallerID, id)
	if err != nil || len(rows) == 0 {
		return false, err
	}
	return rows[0].Merged, nil
}

func (Extra) SetName(ctx context.Context, tallerID, id shared.ID, name string) error {
	_, err := db.Exec(ctx, `UPDATE customers.cliente SET nombre = $3, version = version + 1, updated_at = now()
		WHERE taller_id = $1 AND id = $2`, tallerID, id, name)
	return err
}

func (Extra) SetConsent(ctx context.Context, tallerID, id shared.ID, st domain.ConsentStatus, r app.ConsentRecord) error {
	if _, err := db.Exec(ctx, `UPDATE customers.cliente SET autorizacion_estado = $3, version = version + 1, updated_at = now()
		WHERE taller_id = $1 AND id = $2`, tallerID, id, string(st)); err != nil {
		return err
	}
	var ref *string
	if r.Ref != "" {
		ref = &r.Ref
	}
	_, err := db.Exec(ctx, `INSERT INTO customers.autorizacion (id, taller_id, cliente_id, estado, texto, version_politica, canal, ref, ocurrida_en, usuario_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, shared.NewID(), tallerID, id, string(st), r.Text, r.Version, r.Channel, ref, r.At, r.By)
	return err
}

// Anonymize borra los datos que identifican al cliente (HU-87): nombre, celular y correo. Sus autorizaciones conservan
// el texto aceptado y la fecha (prueba de la autorización), sin la referencia del mensaje.
func (Extra) Anonymize(ctx context.Context, tallerID, id shared.ID, at time.Time) error {
	if _, err := db.Exec(ctx, `UPDATE customers.cliente SET nombre = 'Cliente anonimizado', celular = 'anonimo-' || id::text, correo = NULL,
		anonimizado_en = $3, version = version + 1, updated_at = now() WHERE taller_id = $1 AND id = $2`, tallerID, id, at); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `UPDATE customers.autorizacion SET ref = NULL WHERE taller_id = $1 AND cliente_id = $2`, tallerID, id)
	return err
}

// Anonymized dice si el cliente ya se anonimizó.
func (Extra) Anonymized(ctx context.Context, tallerID, id shared.ID) (bool, error) {
	type r struct {
		Done bool `db:"hecho"`
	}
	rows, err := db.Many[r](ctx, `SELECT anonimizado_en IS NOT NULL AS hecho FROM customers.cliente WHERE taller_id = $1 AND id = $2`, tallerID, id)
	if err != nil || len(rows) == 0 {
		return false, err
	}
	return rows[0].Done, nil
}

// Consents es la historia de autorizaciones del cliente, la más antigua primero (para la copia de sus datos).
func (Extra) Consents(ctx context.Context, tallerID, id shared.ID) ([]app.ConsentEntry, error) {
	type r struct {
		Estado  string    `db:"estado"`
		Texto   string    `db:"texto"`
		Version string    `db:"version_politica"`
		Canal   string    `db:"canal"`
		At      time.Time `db:"ocurrida_en"`
	}
	rows, err := db.Many[r](ctx, `SELECT estado, texto, version_politica, canal, ocurrida_en FROM customers.autorizacion
		WHERE taller_id = $1 AND cliente_id = $2 ORDER BY ocurrida_en`, tallerID, id)
	out := make([]app.ConsentEntry, 0, len(rows))
	for _, x := range rows {
		out = append(out, app.ConsentEntry{State: x.Estado, Text: x.Texto, Version: x.Version, Channel: x.Canal, At: x.At})
	}
	return out, err
}
