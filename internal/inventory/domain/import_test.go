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
	rows, err := ParseImport([]byte(csv))
	if err != nil {
		t.Fatal(err)
	}
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
