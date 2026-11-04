package domain

import (
	"time"

	"sweveh/internal/shared"
)

// Rules son los parámetros del taller que aplican a la cotización. Al crear la cotización se copian el IVA y la
// vigencia: si el taller los cambia después, lo que ya se cotizó no cambia (decisión de Pablo, 3-nov-2026).
type Rules struct {
	VATPct            int
	PricesIncludeVAT  bool
	HourlyRate        *int64
	MaxDiscountPct    int
	ReceptionDiscount bool
	ValidityDays      int
}

// DefaultRules: sin IVA ni vigencia, como antes de los parámetros (pruebas de unidad y cotizaciones viejas).
var DefaultRules = Rules{PricesIncludeVAT: true, MaxDiscountPct: 100, ValidityDays: 15}

var (
	ErrDiscount        = shared.Validation("descuento_invalido", "El descuento va de 0 % hasta el máximo de tu taller.").WithField("descuentoPct")
	ErrDiscountDenied  = shared.Forbidden("descuento_no_permitido", "En tu taller solo el dueño puede dar descuentos.")
	ErrNoRate          = shared.Validation("tarifa_sin_definir", "Define la tarifa por hora en Mi taller → Parámetros, o cotiza la mano de obra con valor.").WithField("horas")
	ErrHours           = shared.Validation("horas_invalidas", "Las horas van de 0,25 a 200, en cuartos de hora.").WithField("horas")
	ErrExpired         = shared.Rule("cotizacion_vencida", "La cotización venció. Renueva la vigencia para registrar la respuesta.")
	ErrRenewNotAllowed = shared.Rule("cotizacion_no_renovable", "Solo se renueva una cotización enviada que espera respuesta.")
)

// Totals es el desglose que ve el cliente: subtotal, descuentos, IVA y total, en pesos.
type Totals struct {
	Gross    int64 // suma de cantidad × precio
	Discount int64
	Base     int64 // Gross − Discount
	VAT      int64
	Total    int64
}

func roundDiv(n, d int64) int64 { return (n + d/2) / d }

// LineDiscount es el descuento de la línea en pesos.
func (l Line) LineDiscount() int64 { return roundDiv(l.Subtotal()*int64(l.DiscountPct), 100) }

// Totals calcula el desglose. Si hay respuesta ítem por ítem, solo cuenta lo aprobado. Con precios que ya incluyen
// IVA, el total es la base y el IVA es lo que va adentro; si no, el IVA se suma.
func (q Quote) Totals() Totals {
	var t Totals
	for _, l := range q.Lines {
		if l.Approved != nil && !*l.Approved {
			continue
		}
		t.Gross += l.Subtotal()
		t.Discount += l.LineDiscount()
	}
	t.Base = t.Gross - t.Discount
	p := int64(q.VATPct)
	if q.PricesIncludeVAT {
		t.Total = t.Base
		t.VAT = t.Total - roundDiv(t.Total*100, 100+p)
	} else {
		t.VAT = roundDiv(t.Base*p, 100)
		t.Total = t.Base + t.VAT
	}
	return t
}

// ApplyRules copia al borrador lo que el taller tiene configurado (IVA y si los precios ya lo incluyen).
func (q *Quote) ApplyRules(r Rules) { q.VATPct, q.PricesIncludeVAT = r.VATPct, r.PricesIncludeVAT }

// SetDiscount pone el descuento de una línea del borrador, hasta el máximo del taller.
func (q *Quote) SetDiscount(id shared.ID, pct, maxPct int) (Line, error) {
	if q.State != Draft {
		return Line{}, ErrNotEditable
	}
	if pct < 0 || pct > maxPct {
		return Line{}, ErrDiscount
	}
	for i := range q.Lines {
		if q.Lines[i].ID == id {
			q.Lines[i].DiscountPct = pct
			return q.Lines[i], nil
		}
	}
	return Line{}, ErrLineNotFound
}

// LaborPrice es horas × tarifa redondeado a pesos; las horas van en centésimas (150 = 1,5 h) y en cuartos de hora.
func LaborPrice(hoursCent int, rate *int64) (int64, error) {
	if rate == nil {
		return 0, ErrNoRate
	}
	if hoursCent < 25 || hoursCent > 20000 || hoursCent%25 != 0 {
		return 0, ErrHours
	}
	return roundDiv(int64(hoursCent)**rate, 100), nil
}

// Expired dice si la cotización enviada ya pasó su vigencia.
func (q Quote) Expired(now time.Time) bool { return q.ExpiresAt != nil && now.After(*q.ExpiresAt) }

// Renew vuelve a contar la vigencia de una cotización enviada (la persona confirmó precios con el cliente).
func (q *Quote) Renew(now time.Time, days int) error {
	if q.State != Sent && q.State != Doubts {
		return ErrRenewNotAllowed
	}
	exp := now.AddDate(0, 0, days)
	q.ExpiresAt = &exp
	return nil
}
