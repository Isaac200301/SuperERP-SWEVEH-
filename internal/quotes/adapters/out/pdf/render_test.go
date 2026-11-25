package pdf

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"sweveh/internal/quotes/app"
	"sweveh/internal/quotes/domain"
)

func yes() *bool { b := true; return &b }

// doc es una cotización ficticia de "Servitaller El Pistón".
func doc(show bool) app.ClientDoc {
	sent := time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC)
	exp := sent.AddDate(0, 0, 15)
	frenos := domain.ClientLine{Kind: domain.KindPartida, Description: "Frenos completos", Qty: 1, UnitPrice: 480000, Subtotal: 480000, DiscountPct: 10, Discount: 48000}
	if show {
		frenos.Detail = []domain.ClientDetail{{Description: "Pastillas delanteras", Qty: 2}, {Description: "Rectificar discos"}}
	}
	return app.ClientDoc{
		Workshop: "Servitaller El Pistón", OrderCode: "OS-0044", Plate: "JNX907", Vehicle: "Mazda 2", CustomerName: "Marta Ejemplo", At: sent,
		Quote: domain.ClientView{Number: 1, State: domain.Sent, SentAt: &sent, ExpiresAt: &exp, ShowDetail: show, VATPct: 19, PricesIncludeVAT: true,
			Lines:  []domain.ClientLine{frenos, {Kind: "servicio", Description: "Alineación y balanceo", Qty: 1, UnitPrice: 60000, Subtotal: 60000, Approved: yes()}},
			Totals: domain.Totals{Gross: 540000, Discount: 48000, Base: 492000, VAT: 78555, Total: 492000}},
	}
}

func TestPesosAndFecha(t *testing.T) {
	for n, want := range map[int64]string{0: "$ 0", 999: "$ 999", 1000: "$ 1.000", 492000: "$ 492.000", 1234567: "$ 1.234.567", -48000: "-$ 48.000"} {
		if got := Pesos(n); got != want {
			t.Errorf("Pesos(%d) = %q, quería %q", n, got, want)
		}
	}
	// 23:30 en UTC del 31 de dic ya es 18:30 del mismo día en Colombia; 02:00 UTC del 1 de ene todavía es 31 de dic.
	if got := Fecha(time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)); got != "31 dic 2025" {
		t.Errorf("fecha en Bogotá: %q", got)
	}
}

func TestRender_ContainsWhatTheClientSees(t *testing.T) {
	b, err := Renderer{}.Render(doc(true))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "%PDF-1.4") {
		t.Fatal("no es un PDF")
	}
	for _, want := range []string{
		"Servitaller El Pist\\363n", "OS-0044", "Mazda 2", "JNX907", "Frenos completos", "Pastillas delanteras \\327 2", "Rectificar discos",
		"$ 492.000", "Total con IVA incluido", "Descuento 10 %", "-$ 48.000", "Firma del cliente", "hasta el 10 dic 2026", "Hoja 1 de 1",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("el PDF no trae %q", want)
		}
	}
}

// El detalle interno y los valores de referencia no salen si el taller no encendió "Mostrar detalle al cliente".
func TestRender_HidesDetailWhenOff(t *testing.T) {
	b, err := Renderer{}.Render(doc(false))
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); strings.Contains(s, "Pastillas") || strings.Contains(s, "Rectificar") {
		t.Fatal("el detalle interno no debía salir")
	}
}

// Una cotización larga sigue en otra hoja sin cortar un ítem a la mitad y numera las hojas.
func TestRender_ManyLinesPaginate(t *testing.T) {
	d := doc(true)
	for i := 0; i < 40; i++ {
		d.Quote.Lines = append(d.Quote.Lines, domain.ClientLine{Kind: "servicio", Description: "Servicio de prueba con un nombre un poco largo para que ocupe dos renglones en la hoja",
			Qty: 2, UnitPrice: 1000, Subtotal: 2000})
	}
	b, err := Renderer{}.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`Hoja 1 de (\d+)`).FindStringSubmatch(string(b))
	if m == nil || m[1] == "1" {
		t.Fatalf("debía seguir en otra hoja: %v", m)
	}
	if !strings.Contains(string(b), "Hoja "+m[1]+" de "+m[1]) {
		t.Fatal("la última hoja no está numerada")
	}
	if os.Getenv("SWEVEH_PDF_OUT") != "" { // para mirarlo a ojo: SWEVEH_PDF_OUT=/ruta/cotizacion.pdf go test -run Paginate
		_ = os.WriteFile(os.Getenv("SWEVEH_PDF_OUT"), b, 0o600)
	}
}

// Texto del usuario que intenta cerrar el literal del PDF e inyectar operadores: sale escapado, sin cerrar el paréntesis.
func TestRender_EscapesHostileText(t *testing.T) {
	d := doc(true)
	d.Workshop = `Taller")TjET(x`
	d.Quote.Lines[0].Description = `Frenos \) (`
	b, err := Renderer{}.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `(Taller"\)TjET\(x) Tj`) {
		t.Fatal("el nombre del taller no salió escapado dentro de su literal")
	}
	if !strings.Contains(s, `(Frenos \\\) \() Tj`) {
		t.Fatal("la descripción no salió escapada dentro de su literal")
	}
}

func TestRender_DraftAndExpired(t *testing.T) {
	d := doc(false)
	d.Quote.State = domain.Draft
	d.Quote.SentAt = nil
	d.Quote.ExpiresAt = nil
	b, err := Renderer{}.Render(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "BORRADOR") || strings.Contains(string(b), "v\\341lida hasta") {
		t.Fatal("un borrador dice que lo es y no tiene vigencia")
	}
	d = doc(false)
	d.Quote.Expired = true
	b, _ = Renderer{}.Render(d)
	if !strings.Contains(string(b), "venci\\363 el") {
		t.Fatal("una cotización vencida lo dice")
	}
}
