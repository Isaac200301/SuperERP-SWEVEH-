package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseImport_ValidatesEachRow(t *testing.T) {
	csv := "\xEF\xBB\xBFreferencia;nombre;tipo;marca;categoria;ubicacion;referencia_oem;costo;precio;existencia;minimo\n" +
		"pf-20;Pastillas delanteras;carro;Bosch;frenos;Estante A3;0986494;40.000;$ 65.000;4;2\n" +
		"FA 10;Filtro de aceite;ambos;;filtros;;;12000;20000;10;3\n" +
		";Sin referencia;;;;;;;1000;;\n" +
		"BH-1;Bombillo;carro;;luces;;;;;;\n" +
		";;;;;;;;;;\n" +
		"PF-20;Repetido;carro;;;;;;5000;;\n"
	file, err := ParseImport([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	rows := file.Rows
	if len(rows) != 5 {
		t.Fatalf("la fila vacía se ignora: %d filas", len(rows))
	}
	p := rows[0].Part
	if len(rows[0].Errors) != 0 || p.Ref != "PF-20" || p.Price != 65000 || p.Cost != 40000 || p.OEM != "0986494" || p.Location != "Estante A3" {
		t.Errorf("fila buena: %+v %v", p, rows[0].Errors)
	}
	if rows[1].Part.Ref != "FA-10" || len(rows[1].Errors) != 0 {
		t.Errorf("la referencia con espacio se normaliza: %+v", rows[1])
	}
	for i, want := range map[int]string{2: "referencia", 3: "categoría", 4: "repetida"} {
		if !strings.Contains(strings.ToLower(strings.Join(rows[i].Errors, " ")), want) {
			t.Errorf("fila %d debía tener error de %s: %v", rows[i].Line, want, rows[i].Errors)
		}
	}
	if !strings.Contains(strings.Join(rows[3].Errors, " "), "precio") {
		t.Errorf("sin precio es error: %v", rows[3].Errors)
	}
}

func TestParseImport_FileLevelErrors(t *testing.T) {
	cases := map[string]error{
		"":                                ErrImportEmpty,
		"referencia,nombre,precio\n":      ErrImportEmpty,
		"nombre,precio\nFiltro,1000\n":    ErrImportHeader,
		strings.Repeat("x", 1<<20+1):      ErrImportTooMany,
		"referencia,nombre,precio\n\"a,b": ErrImportFormat,
	}
	for in, want := range cases {
		if _, err := ParseImport([]byte(in)); !errors.Is(err, want) {
			t.Errorf("%.30q: %v, se esperaba %v", in, err, want)
		}
	}
	var b strings.Builder
	b.WriteString("referencia,nombre,precio\n")
	for i := 0; i <= MaxImportRows; i++ {
		b.WriteString("R")
		b.WriteString(strings.Repeat("1", 1+i%5))
		b.WriteString(",x,1\n")
	}
	if _, err := ParseImport([]byte(b.String())); !errors.Is(err, ErrImportTooMany) {
		t.Errorf("más de %d filas: %v", MaxImportRows, err)
	}
}

func TestPart_SheetFields(t *testing.T) {
	p := Part{Ref: "x1", Name: "Filtro", Brand: "  Mann   Filter ", Category: "FILTROS", OEM: "w 712"}
	if err := p.Normalize(); err != nil || p.Brand != "Mann Filter" || p.Category != "filtros" || p.OEM != "W-712" {
		t.Fatalf("%+v %v", p, err)
	}
	for _, bad := range []Part{
		{Ref: "x", Name: "a", Category: "radios"},
		{Ref: "x", Name: "a", Location: strings.Repeat("a", 41)},
		{Ref: "x", Name: "a", OEM: "-AB"},
	} {
		if err := bad.Normalize(); err == nil {
			t.Errorf("%+v debía rechazarse", bad)
		}
	}
}

// El archivo de prueba de Pablo (3-nov-2026): encabezados de la pantalla, "Para" y categorías en texto legible.
func TestParseImport_HumanHeadersAndValues(t *testing.T) {
	csv := "Referencia,Nombre,Marca,Categoría,Para,Ubicación,Referencia original (OEM),Precio de venta (COP),Costo (COP),Existencia,Mínimo,Notas\n" +
		"FIL-0006,Filtro de aceite moto 150-200cc,Hiflofiltro,Filtros,Moto,A-02-01,15412-KY4-001,18000,11000,30,10,x\n" +
		"LUB-0006,Líquido de frenos DOT 4 500 ml,Bosch,Lubricantes,Carro y moto,B-03-01,,24000,15500,22,8,\n" +
		"SUS-0001,Amortiguador delantero a gas,Monroe,Suspensión,Carro,D-01-01,54302-3X100,\"230,000\",158000,6,2,\n" +
		"CAJ-0001,Kit de embrague,Sachs,Caja y transmisión,Carro,I-01-01,31250-5J010,580000,415000,2,1,\n" +
		"SIN-0001,Tornillería surtida (caja),Genérico,Sin categoría,Carro y moto,,,28000,17000,6,2,\n" +
		"BAD-0001,Precio con decimales,,Otro,Bicicleta,,,1000.50,,,,\n"
	file, err := ParseImport([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Warnings) != 1 || !strings.Contains(file.Warnings[0], "Notas") {
		t.Errorf("la columna desconocida se avisa: %v", file.Warnings)
	}
	want := []struct{ kind, cat string }{{"moto", "filtros"}, {"ambos", "lubricantes"}, {"carro", "suspension"}, {"carro", "transmision"}, {"ambos", ""}}
	for i, w := range want {
		r := file.Rows[i]
		if len(r.Errors) != 0 || r.Part.Kind != w.kind || r.Part.Category != w.cat {
			t.Errorf("fila %d: %+v %v, se esperaba %s/%s", r.Line, r.Part, r.Errors, w.kind, w.cat)
		}
	}
	if file.Rows[2].Part.Price != 230000 || file.Rows[0].Part.Stock != 30 || file.Rows[0].Part.OEM != "15412-KY4-001" {
		t.Errorf("valores: %+v", file.Rows[2].Part)
	}
	if bad := strings.Join(file.Rows[5].Errors, " | "); !strings.Contains(bad, "para:") || !strings.Contains(bad, "precio:") {
		t.Errorf("tipo y precio con decimales son error: %v", bad)
	}
}

func TestParseImport_Windows1252FromExcel(t *testing.T) {
	// "Bujía" y "Eléctrico" como los guarda Excel en Windows: í = 0xED, é = 0xE9.
	data := []byte("Referencia;Nombre;Categor\xeda;Precio\nELE-0003;Buj\xeda de iridio;El\xe9ctrico;38000\n")
	file, err := ParseImport(data)
	if err != nil {
		t.Fatal(err)
	}
	if p := file.Rows[0].Part; p.Name != "Bujía de iridio" || p.Category != "electrico" || len(file.Rows[0].Errors) != 0 {
		t.Errorf("Windows-1252: %+v %v", p, file.Rows[0].Errors)
	}
}
