// Package pdfx escribe PDF sencillos de texto y líneas, tamaño carta, con las fuentes estándar Helvetica.
//
// Es a propósito mínimo (ADR-0043): la cotización solo necesita texto, líneas y recuadros, así que no se agrega una
// librería de PDF (menos superficie de ataque y de licencias, docs/02 §9). Las fuentes estándar no se incrustan: el
// lector las trae, y el texto se codifica en WinAnsi (cp1252), que cubre el español.
package pdfx

import (
	"bytes"
	"fmt"
	"strings"
)

// Tamaño carta en puntos (1 pt = 1/72 de pulgada).
const (
	PageWidth  = 612.0
	PageHeight = 792.0
)

// Doc es un documento en construcción.
type Doc struct{ pages []*Page }

// New crea un documento vacío.
func New() *Doc { return &Doc{} }

// AddPage agrega una hoja carta y la devuelve para dibujar en ella.
func (d *Doc) AddPage() *Page {
	p := &Page{}
	d.pages = append(d.pages, p)
	return p
}

// Pages es la cantidad de hojas.
func (d *Doc) Pages() int { return len(d.pages) }

// Page es una hoja. Las coordenadas van en puntos, con el origen arriba a la izquierda (y crece hacia abajo).
type Page struct{ buf bytes.Buffer }

func pdfY(y float64) float64 { return PageHeight - y }

// Text escribe una línea con su base en (x, y).
func (p *Page) Text(x, y, size float64, bold bool, s string) {
	font := "F1"
	if bold {
		font = "F2"
	}
	fmt.Fprintf(&p.buf, "BT /%s %.2f Tf %.2f %.2f Td (%s) Tj ET\n", font, size, x, pdfY(y), escape(s))
}

// TextRight escribe una línea que termina en xRight.
func (p *Page) TextRight(xRight, y, size float64, bold bool, s string) {
	p.Text(xRight-Width(s, size, bold), y, size, bold, s)
}

// Line traza una línea de grosor w.
func (p *Page) Line(x1, y1, x2, y2, w float64) {
	fmt.Fprintf(&p.buf, "%.2f w %.2f %.2f m %.2f %.2f l S\n", w, x1, pdfY(y1), x2, pdfY(y2))
}

// FillRect pinta un recuadro de gris (0 negro, 1 blanco); (x, y) es la esquina de arriba a la izquierda.
func (p *Page) FillRect(x, y, w, h, gray float64) {
	fmt.Fprintf(&p.buf, "%.2f g %.2f %.2f %.2f %.2f re f 0 g\n", gray, x, pdfY(y+h), w, h)
}

// Bytes arma el archivo PDF con sus tablas de referencias.
func (d *Doc) Bytes() ([]byte, error) {
	if len(d.pages) == 0 {
		return nil, fmt.Errorf("pdfx: el documento no tiene hojas")
	}
	var out bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	// 1 catálogo, 2 páginas, 3 y 4 fuentes; luego, por hoja, su objeto y su contenido.
	var kids []string
	for i := range d.pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+2*i))
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(d.pages)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	for i, pg := range d.pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
			PageWidth, PageHeight, 6+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", pg.buf.Len(), pg.buf.String()))
	}

	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return out.Bytes(), nil
}

// winAnsi lleva un carácter a su byte en cp1252; lo que no existe ahí sale como "?".
func winAnsi(r rune) byte {
	switch {
	case r >= 0x20 && r < 0x7f, r >= 0xa0 && r <= 0xff:
		return byte(r)
	}
	switch r {
	case '€':
		return 0x80
	case '‘':
		return 0x91
	case '’':
		return 0x92
	case '“':
		return 0x93
	case '”':
		return 0x94
	case '•':
		return 0x95
	case '–':
		return 0x96
	case '—':
		return 0x97
	case '−': // signo menos: no está en cp1252
		return '-'
	case '\n', '\r', '\t':
		return ' '
	}
	return '?'
}

// escape codifica el texto en WinAnsi y protege \ ( ) para el literal del PDF; los bytes altos van en octal.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		c := winAnsi(r)
		switch {
		case c == '\\' || c == '(' || c == ')':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x80:
			fmt.Fprintf(&b, "\\%03o", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
