package tenancy

import (
	"context"
	"encoding/json"

	"sweveh/internal/audit"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy/domain"
)

type paramRow struct {
	Esperando  bool            `db:"usar_esperando_repuesto"`
	Calidad    bool            `db:"usar_control_calidad"`
	Fotos      int             `db:"fotos_minimas"`
	Km         bool            `db:"km_obligatorio"`
	ItemsCarro json.RawMessage `db:"items_carro"`
	ItemsMoto  json.RawMessage `db:"items_moto"`
	IVA        int             `db:"iva_pct"`
	ConIVA     bool            `db:"precios_con_iva"`
	Tarifa     *int64          `db:"tarifa_hora"`
	DescMax    int             `db:"descuento_max_pct"`
	DescRecep  bool            `db:"descuento_recepcion"`
	Vigencia   int             `db:"vigencia_dias"`
	Version    int             `db:"version"`
}

const paramCols = `usar_esperando_repuesto, usar_control_calidad, fotos_minimas, km_obligatorio, items_carro, items_moto, iva_pct,
	precios_con_iva, tarifa_hora, descuento_max_pct, descuento_recepcion, vigencia_dias, version`

// ErrParamVersion: otra persona cambió los parámetros mientras se editaban.
var ErrParamVersion = shared.ErrVersionConflict

func (a *Admin) loadParams(ctx context.Context, id shared.ID) (domain.Parametros, error) {
	rows, err := db.Many[paramRow](ctx, `SELECT `+paramCols+` FROM tenancy.parametros WHERE taller_id = $1`, id)
	if err != nil {
		return domain.Parametros{}, err
	}
	if len(rows) == 0 {
		return domain.DefaultParametros(), nil // un taller que no ha configurado nada trabaja con los valores por defecto
	}
	r := rows[0]
	p := domain.Parametros{UseAwaitingParts: r.Esperando, UseQualityCheck: r.Calidad, MinPhotos: r.Fotos, MileageRequired: r.Km,
		VATPct: r.IVA, PricesIncludeVAT: r.ConIVA, HourlyRate: r.Tarifa, MaxDiscountPct: r.DescMax, ReceptionDiscount: r.DescRecep,
		ValidityDays: r.Vigencia, Version: r.Version}
	if err := json.Unmarshal(r.ItemsCarro, &p.CarItems); err != nil {
		return p, err
	}
	if err := json.Unmarshal(r.ItemsMoto, &p.MotoItems); err != nil {
		return p, err
	}
	return p, nil
}

// Parametros devuelve los parámetros del taller (o los de por defecto si nunca los cambió).
func (a *Admin) Parametros(ctx context.Context, id shared.ID) (domain.Parametros, error) {
	var out domain.Parametros
	err := a.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = a.loadParams(ctx, id)
		return err
	})
	return out, err
}

// ParametrosIn los lee dentro de la transacción del llamador (órdenes, cotización, recepción).
func (a *Admin) ParametrosIn(ctx context.Context, id shared.ID) (domain.Parametros, error) {
	return a.loadParams(ctx, id)
}

// SetParametros guarda los parámetros con concurrencia optimista (version 0 = todavía no hay fila) y bitácora.
func (a *Admin) SetParametros(ctx context.Context, id shared.ID, actor shared.Actor, p domain.Parametros, version int) (domain.Parametros, error) {
	if err := p.Validate(); err != nil {
		return domain.Parametros{}, err
	}
	car, err := json.Marshal(p.CarItems)
	if err != nil {
		return domain.Parametros{}, err
	}
	moto, err := json.Marshal(p.MotoItems)
	if err != nil {
		return domain.Parametros{}, err
	}
	var out domain.Parametros
	err = a.uow.Do(ctx, func(ctx context.Context) error {
		cur, err := a.loadParams(ctx, id)
		if err != nil {
			return err
		}
		if cur.Version != version {
			return ErrParamVersion
		}
		n, err := db.Exec(ctx, `INSERT INTO tenancy.parametros (taller_id, `+paramCols+`, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 1, now())
			ON CONFLICT (taller_id) DO UPDATE SET usar_esperando_repuesto = $2, usar_control_calidad = $3, fotos_minimas = $4,
				km_obligatorio = $5, items_carro = $6, items_moto = $7, iva_pct = $8, precios_con_iva = $9, tarifa_hora = $10,
				descuento_max_pct = $11, descuento_recepcion = $12, vigencia_dias = $13,
				version = tenancy.parametros.version + 1, updated_at = now()
			WHERE tenancy.parametros.version = $14`,
			id, p.UseAwaitingParts, p.UseQualityCheck, p.MinPhotos, p.MileageRequired, car, moto, p.VATPct, p.PricesIncludeVAT,
			p.HourlyRate, p.MaxDiscountPct, p.ReceptionDiscount, p.ValidityDays, version)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrParamVersion
		}
		if out, err = a.loadParams(ctx, id); err != nil {
			return err
		}
		return a.audit.Record(ctx, audit.Entry{TallerID: id, Actor: actor, Action: "taller.parametros", Entity: "taller", EntityID: id,
			Before: paramView(cur), After: paramView(out), OccurredAt: a.clock.Now()})
	})
	return out, err
}

func paramView(p domain.Parametros) map[string]any {
	return map[string]any{"esperando_repuesto": p.UseAwaitingParts, "control_calidad": p.UseQualityCheck, "fotos_minimas": p.MinPhotos,
		"km_obligatorio": p.MileageRequired, "items_carro": len(p.CarItems), "items_moto": len(p.MotoItems), "iva_pct": p.VATPct,
		"precios_con_iva": p.PricesIncludeVAT, "tarifa_hora": p.HourlyRate, "descuento_max_pct": p.MaxDiscountPct,
		"descuento_recepcion": p.ReceptionDiscount, "vigencia_dias": p.ValidityDays}
}
