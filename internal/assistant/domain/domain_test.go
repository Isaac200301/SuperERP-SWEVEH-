package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func price(v int64) *int64 { return &v }

var catalog = []Priced{
	{Name: "Revisión de frenos", Price: price(60000), Kind: "servicio", Vehicle: "carro"},
	{Name: "Cambio de pastillas", Price: price(80000), Kind: "servicio", Vehicle: "carro"},
	{Name: "Cambio de amortiguadores", Kind: "servicio", Vehicle: "carro"},
	{Name: "Ajuste de cadena", Price: price(15000), Kind: "servicio", Vehicle: "moto"},
	{Name: "Cambio de aceite", Price: price(45000), Kind: "servicio", Vehicle: "carro"},
	{Name: "Cambio de aceite", Price: price(25000), Kind: "servicio", Vehicle: "moto"},
}

func TestParseUnderstanding_ValidatesSchema(t *testing.T) {
	names := Names(Items(catalog))
	ok := `{"intencion":"cotizar","vehiculo":{"tipo":"carro","marca":"Chevrolet","linea":"Spark GT","anio":2015},"placa":"",
		"sintoma":"chilla al frenar","servicios":["revisión de frenos","Cambio de pastillas","Sincronización"],"otros":[],"fecha":"manana"}`
	u, err := ParseUnderstanding([]byte(ok), "mi Spark GT 2015 chilla al frenar", names)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(u.Services, "|") != "Revisión de frenos|Cambio de pastillas" || strings.Join(u.Others, "|") != "Sincronización" {
		t.Fatalf("servicios = %v, otros = %v", u.Services, u.Others)
	}
	if u.Vehicle.Year != 2015 || u.ForDate != "manana" {
		t.Fatalf("%+v", u)
	}

	bad := map[string]string{
		"no es JSON":          `hola`,
		"campo desconocido":   `{"intencion":"cotizar","precio":45000}`,
		"intención inventada": `{"intencion":"vender","vehiculo":{},"servicios":[],"otros":[]}`,
		"tipo inventado":      `{"intencion":"otro","vehiculo":{"tipo":"bus"}}`,
		"año imposible":       `{"intencion":"otro","vehiculo":{"anio":3015}}`,
		"fecha libre":         `{"intencion":"cita","fecha":"el jueves"}`,
		"dos objetos":         `{"intencion":"otro"}{"intencion":"otro"}`,
		"tipo equivocado":     `{"intencion":"otro","servicios":"frenos"}`,
	}
	for name, raw := range bad {
		if _, err := ParseUnderstanding([]byte(raw), "x", names); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("%s: err = %v, quería ErrInvalidOutput", name, err)
		}
	}
}

func TestParseUnderstanding_PlateMustBeInTheMessage(t *testing.T) {
	raw := `{"intencion":"otro","placa":"klm-482"}`
	u, err := ParseUnderstanding([]byte(raw), "es la KLM482", nil)
	if err != nil || u.Plate != "KLM482" {
		t.Fatalf("placa = %q, %v", u.Plate, err)
	}
	u, _ = ParseUnderstanding([]byte(`{"intencion":"otro","placa":"ZZZ999"}`), "no tengo la placa a mano", nil)
	if u.Plate != "" {
		t.Fatalf("una placa que el cliente no escribió no se usa: %q", u.Plate)
	}
}

func TestInterpret_Rules(t *testing.T) {
	items := Items(catalog)
	names := Names(items)
	u := Interpret("Hola, mi Spark GT 2015 está chillando cuando freno, ¿puedo llevarlo mañana?", items)
	if u.Vehicle != (Vehicle{Type: "carro", Brand: "Chevrolet", Line: "Spark GT", Year: 2015}) {
		t.Fatalf("vehículo = %+v", u.Vehicle)
	}
	if u.ForDate != "manana" || !contains(u.Services, "Revisión de frenos") {
		t.Fatalf("%+v", u)
	}
	for _, text := range []string{"mi NMAX hace ruido en la cadena", "la nave no prende", "cuánto el aceite de la moto"} {
		if v := Interpret(text, items).Vehicle; v.Type != "moto" {
			t.Errorf("%q: tipo = %q", text, v.Type)
		}
	}
	// "motor" no es "moto"; y de una moto no salen servicios que solo son de carro.
	if v := Interpret("se prendió el testigo del motor", items).Vehicle; v.Type != "" {
		t.Errorf("motor: tipo = %q", v.Type)
	}
	if u := Interpret("ajuste de cadena y cambio de pastillas de la moto", items); contains(u.Services, "Cambio de pastillas") || !contains(u.Services, "Ajuste de cadena") {
		t.Errorf("moto: servicios = %v", u.Services)
	}
	for text, kind := range map[string]string{
		"¿qué tiene mi carro? hace un ruido raro": "diagnostico",
		"quiero hablar con una persona":           "humano",
		"escríbeme un poema":                      "fuera_tema",
		"ya puedo ir por la nave?":                "estado",
	} {
		if got := Interpret(text, items).Intent; got != kind {
			t.Errorf("%q: intención %q, quería %q", text, got, kind)
		}
	}
	// La salida de reglas cumple el mismo esquema que la de la IA.
	if _, err := ParseUnderstanding(mustJSON(t, u), "Spark", names); err != nil {
		t.Fatal(err)
	}
}

func TestBuildDraft_OnlyWorkshopData(t *testing.T) {
	in := DraftInput{CustomerName: "Laura Gómez", Services: catalog, Attends: "ambos", NextOpening: "mañana a las 7:30 a. m.",
		Intent: Intent{Kind: "cotizar", ForDate: "manana", Services: []string{"Revisión de frenos", "Cambio de amortiguadores"},
			Others: []string{"Sincronización"}, Vehicle: Vehicle{Type: "carro"}}}
	d := BuildDraft(in)
	for _, want := range []string{"¡Hola Laura!", "$60.000", "amortiguadores el taller te confirma el valor", "mañana a las 7:30", "placa"} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("falta %q en %q", want, d.Text)
		}
	}
	if err := Validate(d.Text, catalog); err != nil {
		t.Fatal(err)
	}
	if !hasSource(d.Sources, "otro", "Sincronización") || !hasSource(d.Sources, "precio", "$60.000") {
		t.Fatalf("fuentes = %+v", d.Sources)
	}

	// Moto: precios de moto (HU-51, criterio 3).
	in.Intent = Intent{Kind: "cotizar", Services: []string{"Cambio de aceite"}, Vehicle: Vehicle{Type: "moto"}}
	if d := BuildDraft(in); !strings.Contains(d.Text, "$25.000") || strings.Contains(d.Text, "$45.000") {
		t.Fatalf("moto: %q", d.Text)
	}
	// Sin tipo: ambos precios, cada uno con su tipo.
	in.Intent.Vehicle.Type = ""
	if d := BuildDraft(in); !strings.Contains(d.Text, "(moto) cuesta $25.000") || !strings.Contains(d.Text, "(carro) cuesta $45.000") {
		t.Fatalf("sin tipo: %q", d.Text)
	}
	// Moto en un taller que solo atiende carros (HU-52, criterio 3).
	in.Attends, in.Intent.Vehicle.Type = "carro", "moto"
	if d := BuildDraft(in); !strings.Contains(d.Text, "solo atiende carros") || strings.Contains(d.Text, "$") {
		t.Fatalf("no atiende: %q", d.Text)
	}
	// Diagnóstico: no se diagnostica, se ofrece la revisión con precio de lista (HU-58, criterio 2).
	in.Attends, in.Intent = "ambos", Intent{Kind: "diagnostico", Vehicle: Vehicle{Type: "carro"}}
	if d := BuildDraft(in); !strings.Contains(d.Text, "no te podemos decir qué tiene") || !strings.Contains(d.Text, "$60.000") {
		t.Fatalf("diagnóstico: %q", d.Text)
	}
}

func TestValidate_RejectsPricesOutsideTheList(t *testing.T) {
	if err := Validate("El cambio de aceite cuesta $45.000.", catalog); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Cuesta $47.000.", "Cuesta $47000", "", strings.Repeat("a", 1001)} {
		if Validate(text, catalog) == nil {
			t.Errorf("%q debería fallar", text)
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func hasSource(src []Source, field, value string) bool {
	for _, s := range src {
		if s.Field == field && s.Value == value {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestGreetSkipsPlaceholderNames(t *testing.T) {
	for name, want := range map[string]string{"Laura Gómez": "¡Hola Laura!", "Cliente de prueba": "¡Hola!", "Cliente WhatsApp": "¡Hola!",
		"": "¡Hola!", "J": "¡Hola!", "+573001234567": "¡Hola!"} {
		if got := Greet("¡Hola", name); got != want {
			t.Errorf("Greet(%q) = %q; quiero %q", name, got, want)
		}
	}
}

// "hola" se responde con lo que hace el asistente; lo que no se entiende no pide la placa.
func TestBuildDraft_GreetingAndUnknown(t *testing.T) {
	d := BuildDraft(DraftInput{CustomerName: "Laura Gómez", Workshop: "Taller El Pistón", Intent: Intent{Kind: "saludo"}})
	if !strings.Contains(d.Text, "¡Hola Laura! Te escribe el asistente de Taller El Pistón") || strings.Contains(d.Text, "placa") {
		t.Fatalf("saludo: %q", d.Text)
	}
	d = BuildDraft(DraftInput{Intent: Intent{Kind: "otro"}})
	if !strings.Contains(d.Text, "no te entendí") || strings.Contains(d.Text, "placa") {
		t.Fatalf("otro: %q", d.Text)
	}
}

// ADR-0029: el texto del asesor solo puede decir precios que trajo una tool, sin enlaces y corto.
func TestGuard(t *testing.T) {
	quoted := map[int64]bool{45000: true, 32000: true}
	for text, ok := range map[string]bool{
		"El cambio de aceite cuesta $45.000 y el aceite $32000.": true,
		"Te sale en $39.000.":              false,
		"Mira www.otrotaller.com":          false,
		"Escríbenos a https://wa.me/57300": false,
		"":                                 false,
		strings.Repeat("a", MaxReply+1):    false,
		"¡Hola! ¿En qué te ayudo con tu vehículo?": true,
	} {
		if err := Guard(text, quoted); (err == nil) != ok {
			t.Errorf("Guard(%.40q) = %v; ok esperado %v", text, err, ok)
		}
	}
}

func TestTidy(t *testing.T) {
	got := Tidy("## Precio\n\nEl **cambio de aceite** cuesta $45.000.\n\n\n\n¿Te agendo?")
	if want := "Precio\n\nEl *cambio de aceite* cuesta $45.000.\n\n¿Te agendo?"; got != want {
		t.Fatalf("Tidy = %q; quiero %q", got, want)
	}
}

func TestPromisesPerson(t *testing.T) {
	for text, want := range map[string]bool{
		"Una persona del taller te va a escribir ya para darte el detalle.": true,
		"Una persona del taller le escribe en cuanto pueda.":                true,
		"El jefe de taller te llama hoy.":                                   true,
		"¿Quieres que te pase con una persona del taller?":                  false,
		"El cambio de aceite cuesta $45.000.":                               false,
	} {
		if got := PromisesPerson(text); got != want {
			t.Errorf("PromisesPerson(%q) = %v", text, got)
		}
	}
}

// ADR-0029: la cita se arma en hora de Colombia y no puede quedar en el pasado ni a más de 30 días.
func TestParseWhen(t *testing.T) {
	now := time.Date(2026, 11, 4, 14, 20, 0, 0, Bogota) // miércoles
	got, err := ParseWhen("2026-11-07", "09:00", now)
	if err != nil || !got.Equal(time.Date(2026, 11, 7, 9, 0, 0, 0, Bogota)) {
		t.Fatalf("sábado 9:00: %v %v", got, err)
	}
	for _, tc := range []struct {
		d, h string
		want error
	}{
		{"2026-11-04", "08:00", ErrWhenPast}, {"2026-12-20", "09:00", ErrWhenFar}, {"7/11/2026", "9am", ErrWhenFormat},
	} {
		if _, err := ParseWhen(tc.d, tc.h, now); !errors.Is(err, tc.want) {
			t.Errorf("%s %s: %v", tc.d, tc.h, err)
		}
	}
	if s := FormatWhen(got); s != "sábado 7 de noviembre, 9:00 a. m." {
		t.Errorf("FormatWhen = %q", s)
	}
	if s := FormatToday(now); s != "miércoles 4 de noviembre de 2026, 2:20 p. m." {
		t.Errorf("FormatToday = %q", s)
	}
	if s := Clock12(time.Date(2026, 1, 1, 0, 5, 0, 0, Bogota)); s != "12:05 a. m." {
		t.Errorf("Clock12 = %q", s)
	}
}
