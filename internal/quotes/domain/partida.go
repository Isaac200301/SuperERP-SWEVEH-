package domain

import (
	"strings"

	"sweveh/internal/shared"
)

// Partidas (V-07, ADR-0040, HU-105): un bloque con nombre y un valor que pone el taller ("Frenos completos $480.000"),
// con un detalle interno opcional. El detalle es informativo: su suma NO cambia el valor de la partida ni el total, y
// el cliente solo lo ve si la cotización tiene "Mostrar detalle al cliente".
const (
	KindPartida    = "partida"
	MaxDetailItems = 40
)

var (
	ErrPartidaValue  = shared.Validation("partida_valor", "La partida necesita nombre y un valor mayor que cero, que pone el taller.").WithField("precioUnitario")
	ErrNotPartida    = shared.Validation("no_es_partida", "El detalle interno es solo para partidas.")
	ErrDetailInvalid = shared.Validation("detalle_invalido", "Cada ítem del detalle necesita descripción de hasta 200 letras, cantidad mayor que cero y valor no negativo; máximo 40.").WithField("detalle")
)

// DetailItem es un renglón del detalle interno de una partida (ValueRef: valor de referencia opcional).
type DetailItem struct {
	Description string
	Qty         int
	ValueRef    *int64
}

// SetDetail reemplaza el detalle interno de una partida del borrador.
func (q *Quote) SetDetail(lineID shared.ID, items []DetailItem) (Line, error) {
	if q.State != Draft {
		return Line{}, ErrNotEditable
	}
	if len(items) > MaxDetailItems {
		return Line{}, ErrDetailInvalid
	}
	clean := make([]DetailItem, 0, len(items))
	for _, it := range items {
		it.Description = strings.Join(strings.Fields(it.Description), " ")
		if it.Qty == 0 {
			it.Qty = 1
		}
		if it.Description == "" || len([]rune(it.Description)) > 200 || it.Qty < 0 || (it.ValueRef != nil && *it.ValueRef < 0) {
			return Line{}, ErrDetailInvalid
		}
		clean = append(clean, it)
	}
	for i := range q.Lines {
		if q.Lines[i].ID != lineID {
			continue
		}
		if q.Lines[i].Kind != KindPartida {
			return Line{}, ErrNotPartida
		}
		q.Lines[i].Detail = clean
		return q.Lines[i], nil
	}
	return Line{}, ErrLineNotFound
}

// Rename cambia el nombre de una partida o de un ítem libre del borrador.
func (q *Quote) Rename(lineID shared.ID, name string) (Line, error) {
	if q.State != Draft {
		return Line{}, ErrNotEditable
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || len([]rune(name)) > 200 {
		return Line{}, ErrLine
	}
	for i := range q.Lines {
		if q.Lines[i].ID == lineID {
			if !q.Lines[i].FreePrice {
				return Line{}, ErrListPrice.WithMessage("Ese nombre sale de la lista del taller; cámbialo en Servicios.")
			}
			q.Lines[i].Description = name
			return q.Lines[i], nil
		}
	}
	return Line{}, ErrLineNotFound
}

// SetShowDetail decide si el cliente ve el detalle de las partidas (solo en borrador: lo que el cliente ya vio no cambia).
func (q *Quote) SetShowDetail(on bool) error {
	if q.State != Draft {
		return ErrNotEditable
	}
	q.ShowDetail = on
	return nil
}
