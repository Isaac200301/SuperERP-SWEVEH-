package pdfx

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func build(t *testing.T, draw func(*Doc)) []byte {
	t.Helper()
	d := New()
	draw(d)
	b, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// El archivo tiene la forma que exige el formato: cabecera, tabla de referencias con los desplazamientos reales y cierre.
func TestBytes_WellFormed(t *testing.T) {
	b := build(t, func(d *Doc) {
		p := d.AddPage()
		p.Text(48, 60, 12, true, "Cotización")
		p.Line(48, 70, 500, 70, 0.5)
		p.FillRect(48, 80, 100, 20, 0.9)
		d.AddPage().Text(48, 60, 10, false, "Segunda hoja")
	})
	if !bytes.HasPrefix(b, []byte("%PDF-1.4")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatalf("cabecera o cierre: %q ... %q", b[:10], b[len(b)-10:])
	}
	sx := regexp.MustCompile(`startxref\n(\d+)\n`).FindSubmatch(b)
	pos, _ := strconv.Atoi(string(sx[1]))
	if !bytes.HasPrefix(b[pos:], []byte("xref\n0 ")) {
		t.Fatalf("startxref no apunta a la tabla: %q", b[pos:pos+10])
	}
	// Cada entrada de la tabla apunta al "N 0 obj" que dice ser.
	entries := regexp.MustCompile(`(\d{10}) 00000 n \n`).FindAllSubmatch(b[pos:], -1)
	if len(entries) != 8 { // catálogo, páginas, 2 fuentes y 2 × (hoja + contenido)
		t.Fatalf("objetos = %d, quería 8", len(entries))
	}
	for i, e := range entries {
		off, _ := strconv.Atoi(string(e[1]))
		if !bytes.HasPrefix(b[off:], []byte(fmt.Sprintf("%d 0 obj", i+1))) {
			t.Fatalf("el objeto %d no está en %d: %q", i+1, off, b[off:off+12])
		}
	}
	if !strings.Contains(string(b), "/Count 2") {
		t.Fatal("el árbol de páginas no cuenta 2")
	}
}

func TestBytes_EmptyDocFails(t *testing.T) {
	if _, err := New().Bytes(); err == nil {
		t.Fatal("un documento sin hojas no se escribe")
	}
}

// Las tildes y la eñe salen en WinAnsi (octal) y los paréntesis y la barra se protegen.
func TestEscape(t *testing.T) {
	for in, want := range map[string]string{
		"Cotización":   `Cotizaci\363n`,
		"Año (2026)":   `A\361o \(2026\)`,
		`a\b`:          `a\\b`,
		"–":            `\226`,
		"−$ 5":         "-$ 5",
		"日本":           "??",
		"línea\nnueva": "l\\355nea nueva",
	} {
		if got := escape(in); got != want {
			t.Errorf("escape(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestWidthAndWrap(t *testing.T) {
	if w := Width("Hola", 10, false); w != (722+556+222+556)*10/1000.0 {
		t.Fatalf("ancho de Hola = %v", w)
	}
	if Width("áéñ", 10, false) != Width("aen", 10, false) {
		t.Fatal("una letra con tilde mide lo mismo que sin ella")
	}
	lines := Wrap("Cambio de aceite y filtro con revisión de niveles completa", 10, false, 120)
	if len(lines) < 3 {
		t.Fatalf("debía partirse: %q", lines)
	}
	for _, l := range lines {
		if Width(l, 10, false) > 120 {
			t.Fatalf("la línea %q pasa del ancho", l)
		}
	}
	if joined := strings.Join(lines, " "); joined != "Cambio de aceite y filtro con revisión de niveles completa" {
		t.Fatalf("se perdió texto: %q", joined)
	}
	// Una palabra más larga que la línea se corta sin dañar las letras de varios bytes.
	long := Wrap("ñññññññññññññññññññññññññññññ", 10, false, 40)
	if len(long) < 2 || strings.Join(long, "") != "ñññññññññññññññññññññññññññññ" {
		t.Fatalf("palabra larga: %q", long)
	}
	if got := Wrap("   ", 10, false, 100); len(got) != 1 || got[0] != "" {
		t.Fatalf("texto vacío: %q", got)
	}
}
