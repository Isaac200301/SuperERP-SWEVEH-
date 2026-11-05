package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"sweveh/internal/shared"
)

// HU-92: la plantilla con columna de tipo valida cada placa según sea carro o moto y marca las filas con error; sin
// cilindraje o turbo el vehículo entra con motor pendiente (no se rechaza la fila).
func TestParseImport(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	csv := "Nombre;Celular;Placa;Tipo de vehículo;Marca;Línea;Modelo;Cilindraje;Turbo;Color favorito\n" +
		"Carlos Ruiz;300 123 4567;abc-123;Carro;Mazda;3;2015;2000;no;rojo\n" + // completo
		"Carlos Ruiz;3001234567;GNO66F;moto;Yamaha;NMAX;2022;;;\n" + // mismo cliente, moto sin motor: pendiente
		"Laura Gómez;3101234567;XYZ12A;carro;;;;;;\n" + // placa de moto marcada como carro
		";3201234567;JKL456;carro;;;;;;\n" + // sin nombre
		"Ana Ríos;12345;MNO789;moto;;;;;;\n" + // celular malo; además MNO789 es de carro
		"Pedro Gil;3151234567;ABC123;carro;;;;;;\n" + // placa repetida en el archivo
		"Carla Ruiz;3001234567;QRS456;carro;;;;;quizás\n" + // celular con otro nombre y turbo inválido
		"Luis Mora;3161234567;TUV789;bus;;;1890;;;\n" // tipo inválido y año fuera de rango

	f, err := ParseImport([]byte(csv), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Warnings) != 1 || !strings.Contains(f.Warnings[0], "Color favorito") {
		t.Fatalf("avisos: %v", f.Warnings)
	}
	want := map[int][]string{
		2: nil,
		3: nil,
		4: {"es de moto, no de carro"},
		5: {"falta el nombre"},
		6: {"no es un celular de Colombia", "es de carro, no de moto"},
		7: {"ya está en la fila 2"},
		8: {"con otro nombre (Carlos Ruiz)", "turbo escribe sí o no"},
		9: {"debe ser carro o moto"},
	}
	for _, r := range f.Rows {
		exp := want[r.Line]
		if len(exp) == 0 && len(r.Errors) > 0 {
			t.Errorf("fila %d sin errores esperados: %v", r.Line, r.Errors)
		}
		joined := strings.Join(r.Errors, " | ")
		for _, e := range exp {
			if !strings.Contains(joined, e) {
				t.Errorf("fila %d: quiero %q en %q", r.Line, e, joined)
			}
		}
	}
	car, moto := f.Rows[0], f.Rows[1]
	if car.Phone != "+573001234567" || car.Input.Plate.String() != "ABC123" || car.Input.Kind != "carro" || *car.Input.Details.Brand != "Mazda" ||
		*car.Input.Engine.DisplacementCC != 2000 || *car.Input.Engine.Turbo {
		t.Fatalf("fila completa: %+v", car)
	}
	v, err := NewVehicle(shared.NewID(), shared.NewID(), moto.Input, now)
	if err != nil || !v.Engine.Pending(v.Kind) {
		t.Fatalf("la moto sin motor debe quedar pendiente: %v %v", err, v.Engine)
	}
}

func TestParseImport_FileErrors(t *testing.T) {
	now := time.Now()
	for name, c := range map[string]struct {
		data string
		want error
	}{
		"sin columnas obligatorias": {"Nombre;Celular;Placa\nCarlos;3001234567;ABC123\n", ErrImportHeader},
		"sin filas":                 {"Nombre;Celular;Placa;Tipo\n\n", ErrImportEmpty},
		"vacío":                     {"", ErrImportEmpty},
	} {
		if _, err := ParseImport([]byte(c.data), now); !errors.Is(err, c.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// La plantilla que descarga la pantalla Clientes (web/src/features/customers/CustomersPage.tsx) se lee sin errores ni
// avisos: si cambia un encabezado allá, esta prueba lo dice.
func TestParseImport_WebTemplate(t *testing.T) {
	tpl := "Nombre;Celular;Placa;Tipo de vehículo;Marca;Línea;Modelo;Kilometraje;Cilindraje;Turbo;Energía;Caja\n" +
		"Carlos Ruiz;3001234567;ABC123;Carro;Mazda;3;2015;85000;2000;No;Gasolina;Manual\n" +
		"Carlos Ruiz;3001234567;GNO66F;Moto;Yamaha;NMAX;2022;;;;;\n"
	f, err := ParseImport([]byte(tpl), time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if err != nil || len(f.Warnings) > 0 {
		t.Fatalf("plantilla: %v %v", err, f.Warnings)
	}
	for _, r := range f.Rows {
		if len(r.Errors) > 0 {
			t.Errorf("fila %d: %v", r.Line, r.Errors)
		}
	}
	v, err := NewVehicle(shared.NewID(), shared.NewID(), f.Rows[0].Input, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if err != nil || v.Engine.Pending(v.Kind) {
		t.Fatalf("el carro completo no queda pendiente: %v %+v", err, v.Engine)
	}
}
