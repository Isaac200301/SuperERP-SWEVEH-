// Package pdf escribe la cotización en PDF tamaño carta para compartirla con el cliente (V-10, HU-109, ADR-0043).
package pdf

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"sweveh/internal/platform/pdfx"
	"sweveh/internal/quotes/app"
	"sweveh/internal/quotes/domain"
)

// bogota: Colombia no tiene horario de verano, así que un desfase fijo es exacto y no depende de tzdata.
var bogota = time.FixedZone("America/Bogota", -5*60*60)

var months = [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

// Medidas de la hoja en puntos.
const (
	left, right = 48.0, 564.0
	descW       = 290.0
	colQty      = 392.0
	colUnit     = 482.0
	bottom      = 700.0
	lineH       = 13.0
)

// Renderer escribe el PDF de la cotización.
type Renderer struct{}

var _ app.Renderer = Renderer{}

// Pesos formatea pesos colombianos sin decimales y con punto de miles: $ 1.234.000.
func Pesos(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return sign + "$ " + b.String()
}

// Fecha es "25 nov 2026" en hora de Colombia.
func Fecha(t time.Time) string {
	t = t.In(bogota)
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

// layout lleva la hoja actual y la línea donde se escribe.
type layout struct {
	doc   *pdfx.Doc
	pages []*pdfx.Page
	page  *pdfx.Page
	y     float64
}

func (l *layout) newPage() {
	l.page = l.doc.AddPage()
	l.pages = append(l.pages, l.page)
	l.y = 60
}

func (l *layout) need(h float64) {
	if l.y+h > bottom {
		l.newPage()
	}
}

// Render arma el PDF: encabezado, tabla de ítems, totales, vigencia y espacio para la firma.
func (Renderer) Render(d app.ClientDoc) ([]byte, error) {
	l := &layout{doc: pdfx.New()}
	l.newPage()
	l.header(d)
	l.table(d.Quote)
	l.totals(d.Quote)
	l.validity(d.Quote)
	l.signature()
	for i, pg := range l.pages {
		pg.Text(left, 760, 8, false, "Valores en pesos colombianos (COP).")
		pg.TextRight(right, 760, 8, false, fmt.Sprintf("Hoja %d de %d", i+1, len(l.pages)))
	}
	return l.doc.Bytes()
}

func (l *layout) header(d app.ClientDoc) {
	name := pdfx.Wrap(d.Workshop, 18, true, 330)
	for i, s := range name {
		l.page.Text(left, l.y+float64(i)*22, 18, true, s)
	}
	date := d.At
	if d.Quote.SentAt != nil {
		date = *d.Quote.SentAt
	}
	l.page.TextRight(right, l.y-4, 11, true, "COTIZACIÓN")
	l.page.TextRight(right, l.y+12, 10, false, fmt.Sprintf("Orden %s · Cotización %d", d.OrderCode, d.Quote.Number))
	l.page.TextRight(right, l.y+26, 10, false, "Fecha: "+Fecha(date))
	l.y += float64(len(name))*22 + 20
	l.page.Line(left, l.y, right, l.y, 0.8)
	l.y += 24

	l.field("Vehículo", strings.TrimSpace(d.Vehicle+" · placa "+d.Plate))
	if d.CustomerName != "" {
		l.field("Cliente", d.CustomerName)
	}
	if d.Quote.State == domain.Draft {
		l.page.Text(left, l.y, 10, true, "BORRADOR · todavía no se ha enviado")
		l.y += 16
	}
	l.y += 12
}

// field escribe "Etiqueta  valor" y parte el valor en renglones si no cabe en la hoja.
func (l *layout) field(label, value string) {
	l.page.Text(left, l.y, 10, true, label)
	for _, s := range pdfx.Wrap(value, 10, false, right-left-60) {
		l.page.Text(left+60, l.y, 10, false, s)
		l.y += 14
	}
	l.y += 2
}

func (l *layout) tableHead() {
	l.page.FillRect(left, l.y-12, right-left, 20, 0.92)
	l.page.Text(left+6, l.y+2, 9, true, "Descripción")
	l.page.TextRight(colQty, l.y+2, 9, true, "Cant.")
	l.page.TextRight(colUnit, l.y+2, 9, true, "Valor unitario")
	l.page.TextRight(right-6, l.y+2, 9, true, "Subtotal")
	l.y += 24
}

func (l *layout) table(v domain.ClientView) {
	l.tableHead()
	for _, ln := range v.Lines {
		l.row(ln)
	}
}

// rowHeight calcula cuánto ocupa un ítem para no partirlo entre dos hojas.
func rowHeight(ln domain.ClientLine) float64 {
	h := float64(len(pdfx.Wrap(ln.Description, 10, ln.Kind == domain.KindPartida, descW))) * lineH
	for _, d := range ln.Detail {
		h += float64(len(pdfx.Wrap(detailText(d), 9, false, descW-14))) * 12
	}
	if ln.DiscountPct > 0 {
		h += 12
	}
	return h + 8
}

func detailText(d domain.ClientDetail) string {
	if d.Qty > 1 {
		return fmt.Sprintf("%s × %d", d.Description, d.Qty)
	}
	return d.Description
}

func (l *layout) row(ln domain.ClientLine) {
	l.need(rowHeight(ln))
	partida := ln.Kind == domain.KindPartida
	desc := ln.Description
	if ln.Approved != nil && !*ln.Approved {
		desc += " (no aprobado)"
	}
	top := l.y
	for _, s := range pdfx.Wrap(desc, 10, partida, descW) {
		l.page.Text(left+6, l.y, 10, partida, s)
		l.y += lineH
	}
	for _, d := range ln.Detail {
		for i, s := range pdfx.Wrap(detailText(d), 9, false, descW-14) {
			if i == 0 {
				s = "• " + s
			} else {
				s = "  " + s
			}
			l.page.Text(left+18, l.y-1, 9, false, s)
			l.y += 12
		}
	}
	if ln.DiscountPct > 0 {
		l.page.Text(left+18, l.y-1, 9, false, fmt.Sprintf("Descuento %d %%", ln.DiscountPct))
		l.page.TextRight(right-6, l.y-1, 9, false, "-"+Pesos(ln.Discount))
		l.y += 12
	}
	l.page.TextRight(colQty, top, 10, false, strconv.Itoa(ln.Qty))
	l.page.TextRight(colUnit, top, 10, false, Pesos(ln.UnitPrice))
	l.page.TextRight(right-6, top, 10, false, Pesos(ln.Subtotal))
	l.y += 4
	l.page.Line(left, l.y, right, l.y, 0.3)
	l.y += 14
}

func (l *layout) totals(v domain.ClientView) {
	t := v.Totals
	l.need(90)
	l.y += 4
	row := func(label string, value string, bold bool, size float64) {
		l.page.Text(colQty-70, l.y, size, bold, label)
		l.page.TextRight(right-6, l.y, size, bold, value)
		l.y += size + 6
	}
	if t.Discount > 0 || (v.VATPct > 0 && !v.PricesIncludeVAT) {
		row("Subtotal", Pesos(t.Gross), false, 10)
	}
	if t.Discount > 0 {
		row("Descuentos", "-"+Pesos(t.Discount), false, 10)
	}
	if v.VATPct > 0 {
		label := fmt.Sprintf("IVA %d %%", v.VATPct)
		if v.PricesIncludeVAT {
			label += " (incluido)"
		}
		row(label, Pesos(t.VAT), false, 10)
	}
	l.page.Line(colQty-70, l.y-6, right-6, l.y-6, 0.8)
	l.y += 6
	label := "Total"
	if v.PricesIncludeVAT && v.VATPct > 0 {
		label = "Total con IVA incluido"
	}
	l.page.Text(colQty-70, l.y, 12, true, label)
	l.page.TextRight(right-6, l.y, 12, true, Pesos(t.Total))
	l.y += 30
}

func (l *layout) validity(v domain.ClientView) {
	if v.ExpiresAt == nil {
		return
	}
	l.need(24)
	s := "Esta cotización es válida hasta el " + Fecha(*v.ExpiresAt) + "."
	if v.Expired {
		s = "Esta cotización venció el " + Fecha(*v.ExpiresAt) + "."
	}
	l.page.Text(left, l.y, 10, false, s)
	l.y += 28
}

func (l *layout) signature() {
	l.need(110)
	l.y += 20
	l.page.Text(left, l.y, 10, false, "Acepto esta cotización y autorizo el trabajo descrito.")
	l.y += 52
	l.page.Line(left, l.y, 280, l.y, 0.6)
	l.page.Line(330, l.y, right, l.y, 0.6)
	l.page.Text(left, l.y+14, 9, false, "Firma del cliente")
	l.page.Text(330, l.y+14, 9, false, "Fecha")
}
