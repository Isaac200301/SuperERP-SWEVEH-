package postgres

import (
	"context"
	"encoding/json"
	"time"

	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Flow implementa ports.FlowRepository con pgx (ADR-0024).
type Flow struct{}

type orderRow struct {
	ID         shared.ID  `db:"id"`
	TallerID   shared.ID  `db:"taller_id"`
	Numero     int        `db:"numero"`
	VehiculoID shared.ID  `db:"vehiculo_id"`
	ClienteID  shared.ID  `db:"cliente_id"`
	Placa      string     `db:"placa"`
	Tipo       string     `db:"tipo_vehiculo"`
	Estado     string     `db:"estado"`
	Sintoma    string     `db:"sintoma"`
	Version    int        `db:"version"`
	CreatedAt  time.Time  `db:"created_at"`
	CreatedBy  *shared.ID `db:"created_by"`
	MecanicoID *shared.ID `db:"mecanico_id"`
	Prometida  *time.Time `db:"fecha_prometida"`
	Ejemplo    bool       `db:"ejemplo"`
	EntregadoA *string    `db:"entregado_a"`
	MotivoCanc *string    `db:"motivo_cancelacion"`
	CerradaEn  *time.Time `db:"cerrada_en"`
}

const orderCols = `id, taller_id, numero, vehiculo_id, cliente_id, placa, tipo_vehiculo, estado, sintoma, version, created_at,
	created_by, mecanico_id, fecha_prometida, ejemplo, entregado_a, motivo_cancelacion, cerrada_en`

func (r orderRow) domain() domain.Order {
	o := domain.Order{ID: r.ID, TallerID: r.TallerID, Number: r.Numero, VehicleID: r.VehiculoID, CustomerID: r.ClienteID,
		Plate: r.Placa, VehicleKind: r.Tipo, State: domain.State(r.Estado), Symptom: r.Sintoma, Version: r.Version,
		CreatedAt: r.CreatedAt, MechanicID: r.MecanicoID, PromisedAt: r.Prometida, Example: r.Ejemplo, ClosedAt: r.CerradaEn}
	if r.CreatedBy != nil {
		o.CreatedBy = *r.CreatedBy
	}
	if r.EntregadoA != nil {
		o.PickedUpBy = *r.EntregadoA
	}
	if r.MotivoCanc != nil {
		o.CancelReason = *r.MotivoCanc
	}
	return o
}

func (Flow) Load(ctx context.Context, tallerID, id shared.ID) (domain.Order, error) {
	r, err := db.One[orderRow](ctx, domain.ErrOrderNotFound, `SELECT `+orderCols+` FROM orders.orden WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return r.domain(), err
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (Flow) SaveState(ctx context.Context, o domain.Order, prev int) error {
	n, err := db.Exec(ctx, `UPDATE orders.orden SET estado = $3, version = $4, entregado_a = $5, motivo_cancelacion = $6,
		cerrada_en = $7, updated_at = now() WHERE taller_id = $1 AND id = $2 AND version = $8`,
		o.TallerID, o.ID, string(o.State), o.Version, nilIfEmpty(o.PickedUpBy), nilIfEmpty(o.CancelReason), o.ClosedAt, prev)
	if err == nil && n == 0 {
		return shared.ErrVersionConflict
	}
	return err
}

func (Flow) SaveAssignment(ctx context.Context, o domain.Order) error {
	_, err := db.Exec(ctx, `UPDATE orders.orden SET mecanico_id = $3, fecha_prometida = $4, updated_at = now()
		WHERE taller_id = $1 AND id = $2`, o.TallerID, o.ID, o.MechanicID, o.PromisedAt)
	return err
}

func (Flow) MarkExample(ctx context.Context, tallerID, id shared.ID) error {
	_, err := db.Exec(ctx, `UPDATE orders.orden SET ejemplo = true WHERE taller_id = $1 AND id = $2`, tallerID, id)
	return err
}

func (Flow) InsertHistory(ctx context.Context, tallerID, orderID shared.ID, from *domain.State, to domain.State, by *shared.ID, reason string, at time.Time) error {
	var desde *string
	if from != nil {
		s := string(*from)
		desde = &s
	}
	_, err := db.Exec(ctx, `INSERT INTO orders.estado_historial (id, taller_id, orden_id, desde, hacia, usuario_id, motivo, ocurrido_en)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, shared.NewID(), tallerID, orderID, desde, string(to), by, nilIfEmpty(reason), at)
	return err
}

func (Flow) List(ctx context.Context, f ports.ListFilter) ([]domain.Order, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 50
	}
	var st *string
	if f.State != nil {
		s := string(*f.State)
		st = &s
	}
	var cAt *time.Time
	var cID *shared.ID
	if f.Cursor != nil {
		cAt, cID = &f.Cursor.CreatedAt, &f.Cursor.ID
	}
	// Con mecánico: primero por fecha prometida (HU-23); si no, las más recientes.
	rows, err := db.Many[orderRow](ctx, `SELECT `+orderCols+` FROM orders.orden
		WHERE taller_id = $1
		  AND (NOT $2 OR estado NOT IN ('entregado', 'cancelada'))
		  AND ($3::text IS NULL OR estado = $3)
		  AND ($4::uuid IS NULL OR vehiculo_id = $4)
		  AND ($5::uuid IS NULL OR mecanico_id = $5)
		  AND ($6::timestamptz IS NULL OR (created_at, id) < ($6, $7::uuid))
		ORDER BY CASE WHEN $5::uuid IS NOT NULL THEN fecha_prometida END ASC NULLS LAST, created_at DESC, id DESC
		LIMIT $8`, f.TallerID, f.OnlyOpen, st, f.VehicleID, f.MechanicID, cAt, cID, f.Limit)
	out := make([]domain.Order, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.domain())
	}
	return out, err
}

type checklistRow struct {
	Tipo        string          `db:"tipo"`
	Km          *int            `db:"kilometraje"`
	Combustible *string         `db:"combustible"`
	Accesorios  []string        `db:"accesorios"`
	Salvedades  json.RawMessage `db:"salvedades"`
	Por         *shared.ID      `db:"completado_por"`
	En          *time.Time      `db:"completado_en"`
	Version     int             `db:"version"`
}

func (Flow) GetChecklist(ctx context.Context, tallerID, orderID shared.ID) (domain.Checklist, bool, error) {
	rows, err := db.Many[checklistRow](ctx, `SELECT tipo, kilometraje, combustible, accesorios, salvedades, completado_por, completado_en, version
		FROM orders.checklist WHERE taller_id = $1 AND orden_id = $2`, tallerID, orderID)
	if err != nil || len(rows) == 0 {
		return domain.Checklist{OrderID: orderID, Accessories: []string{}}, false, err
	}
	r := rows[0]
	c := domain.Checklist{OrderID: orderID, Kind: r.Tipo, Km: r.Km, Fuel: r.Combustible, Accessories: r.Accesorios,
		DoneBy: r.Por, DoneAt: r.En, Version: r.Version}
	if err := json.Unmarshal(r.Salvedades, &c.Exceptions); err != nil {
		return c, true, err
	}
	return c, true, nil
}

func (Flow) SaveChecklist(ctx context.Context, tallerID shared.ID, c domain.Checklist) error {
	exc, err := json.Marshal(c.Exceptions)
	if err != nil {
		return err
	}
	if c.Exceptions == nil {
		exc = []byte("[]")
	}
	if c.Accessories == nil {
		c.Accessories = []string{}
	}
	_, err = db.Exec(ctx, `INSERT INTO orders.checklist (orden_id, taller_id, tipo, kilometraje, combustible, accesorios, salvedades,
		completado_por, completado_en) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (orden_id) DO UPDATE SET kilometraje = EXCLUDED.kilometraje, combustible = EXCLUDED.combustible,
		accesorios = EXCLUDED.accesorios, salvedades = EXCLUDED.salvedades, completado_por = EXCLUDED.completado_por,
		completado_en = EXCLUDED.completado_en, version = orders.checklist.version + 1, updated_at = now()`,
		c.OrderID, tallerID, c.Kind, c.Km, c.Fuel, c.Accessories, exc, c.DoneBy, c.DoneAt)
	return err
}

type evidenceRow struct {
	ID          shared.ID  `db:"id"`
	TallerID    shared.ID  `db:"taller_id"`
	OrdenID     shared.ID  `db:"orden_id"`
	Lado        *string    `db:"lado"`
	Nota        *string    `db:"nota"`
	StorageKey  *string    `db:"storage_key"`
	ContentType *string    `db:"content_type"`
	Mostrar     bool       `db:"mostrar_cliente"`
	Etapa       string     `db:"etapa"`
	AutorID     *shared.ID `db:"autor_id"`
	CreatedAt   time.Time  `db:"created_at"`
}

func (r evidenceRow) port() ports.Evidence {
	return ports.Evidence{ID: r.ID, TallerID: r.TallerID, OrderID: r.OrdenID, Side: r.Lado, Note: r.Nota, StorageKey: r.StorageKey,
		ContentType: r.ContentType, ShowClient: r.Mostrar, Stage: r.Etapa, AuthorID: r.AutorID, CreatedAt: r.CreatedAt}
}

const evCols = `id, taller_id, orden_id, lado, nota, storage_key, content_type, mostrar_cliente, etapa, autor_id, created_at`

func (Flow) InsertEvidence(ctx context.Context, e ports.Evidence) error {
	_, err := db.Exec(ctx, `INSERT INTO orders.evidencia (`+evCols+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		e.ID, e.TallerID, e.OrderID, e.Side, e.Note, e.StorageKey, e.ContentType, e.ShowClient, e.Stage, e.AuthorID, e.CreatedAt)
	return err
}

func (Flow) ListEvidence(ctx context.Context, tallerID, orderID shared.ID) ([]ports.Evidence, error) {
	rows, err := db.Many[evidenceRow](ctx, `SELECT `+evCols+` FROM orders.evidencia WHERE taller_id = $1 AND orden_id = $2
		ORDER BY created_at, id`, tallerID, orderID)
	out := make([]ports.Evidence, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.port())
	}
	return out, err
}

func (Flow) GetEvidence(ctx context.Context, tallerID, orderID, id shared.ID) (ports.Evidence, error) {
	r, err := db.One[evidenceRow](ctx, shared.NotFound("evidencia_no_encontrada", "No encontramos esa foto en la orden."),
		`SELECT `+evCols+` FROM orders.evidencia WHERE taller_id = $1 AND orden_id = $2 AND id = $3`, tallerID, orderID, id)
	return r.port(), err
}

func (Flow) PhotoSides(ctx context.Context, tallerID, orderID shared.ID) ([]string, error) {
	type s struct {
		Lado string `db:"lado"`
	}
	rows, err := db.Many[s](ctx, `SELECT DISTINCT lado FROM orders.evidencia WHERE taller_id = $1 AND orden_id = $2
		AND storage_key IS NOT NULL AND lado IS NOT NULL`, tallerID, orderID)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Lado)
	}
	return out, err
}
