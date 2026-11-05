package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"sweveh/internal/shared"
	"sweveh/internal/shared/csvx"
)

// Límites de la importación de clientes y vehículos (HU-92), los mismos del inventario (HU-37).
const (
	MaxImportRows  = 1000
	MaxImportBytes = 1 << 20
)

var (
	ErrImportEmpty   = shared.Validation("importacion_vacia", "El archivo no tiene filas. Usa la plantilla y llena al menos una fila.").WithField("archivo")
	ErrImportHeader  = shared.Validation("importacion_columnas", "Faltan columnas obligatorias: nombre, celular, placa y tipo de vehículo.").WithField("archivo")
	ErrImportTooMany = shared.Validation("importacion_grande", "El archivo tiene más de 1.000 filas o pesa más de 1 MB. Divídelo en partes.").WithField("archivo")
	ErrImportFormat  = shared.Validation("importacion_formato", "No pudimos leer el archivo. Guárdalo como CSV (separado por comas o punto y coma).").WithField("archivo")
)

// importAliases traduce los encabezados de la plantilla (o de un Excel hecho a mano) a la columna interna.
var importAliases = map[string]string{
	"nombre": "nombre", "cliente": "nombre", "nombre del cliente": "nombre", "propietario": "nombre",
	"celular": "celular", "telefono": "celular", "whatsapp": "celular", "movil": "celular",
	"placa": "placa",
	"tipo":  "tipo", "tipo de vehiculo": "tipo", "vehiculo": "tipo", "carro o moto": "tipo",
	"marca": "marca",
	"linea": "linea", "referencia": "linea",
	"modelo": "modelo", "ano": "modelo", "año": "modelo", "modelo ano": "modelo",
	"kilometraje": "kilometraje", "km": "kilometraje",
	"cilindraje": "cilindraje", "cc": "cilindraje",
	"turbo":   "turbo",
	"energia": "energia", "combustible": "energia",
	"caja": "caja", "transmision": "caja", "caja de cambios": "caja",
}

var importKinds = map[string]Kind{"carro": KindCar, "carros": KindCar, "auto": KindCar, "automovil": KindCar, "camioneta": KindCar,
	"moto": KindMoto, "motos": KindMoto, "motocicleta": KindMoto}

// ImportRow es una fila ya validada: Errors vacío = se puede importar.
type ImportRow struct {
	Line   int // número de fila en el archivo, contando el encabezado como 1
	Name   string
	Phone  string // celular en formato +57
	Input  NewVehicleInput
	Errors []string
}

// ImportFile son las filas y los avisos que no impiden importar (columnas desconocidas).
type ImportFile struct {
	Rows     []ImportRow
	Warnings []string
}

// ParseImport lee la plantilla de clientes y vehículos y valida cada fila con las reglas del vehículo (HU-92):
// la placa según sea carro o moto, y sin cilindraje o turbo el vehículo entra con "motor pendiente" (no es error).
// No guarda nada; el caso de uso decide si confirma.
func ParseImport(data []byte, now time.Time) (ImportFile, error) {
	t, err := csvx.Read(data, MaxImportBytes, MaxImportRows)
	switch {
	case errors.Is(err, csvx.ErrTooLarge):
		return ImportFile{}, ErrImportTooMany
	case errors.Is(err, csvx.ErrEmpty):
		return ImportFile{}, ErrImportEmpty
	case err != nil:
		return ImportFile{}, ErrImportFormat
	}
	idx, warnings := csvx.Columns(t.Header, importAliases)
	for _, need := range []string{"nombre", "celular", "placa", "tipo"} {
		if _, ok := idx[need]; !ok {
			return ImportFile{}, ErrImportHeader
		}
	}
	out := ImportFile{Warnings: warnings}
	for _, r := range t.Rows {
		out.Rows = append(out.Rows, parseImportRow(r.Line, r.Fields, idx, now))
	}
	markImportDuplicates(out.Rows)
	return out, nil
}

func parseImportRow(line int, rec []string, idx map[string]int, now time.Time) ImportRow {
	get := func(col string) string { return csvx.Cell(rec, idx, col) }
	row := ImportRow{Line: line, Name: strings.Join(strings.Fields(get("nombre")), " ")}
	fail := func(msg string) { row.Errors = append(row.Errors, msg) }
	if row.Name == "" {
		fail("falta el nombre del cliente")
	} else if len([]rune(row.Name)) > 120 {
		fail("el nombre tiene más de 120 letras")
	}
	if phone, err := shared.ParseColombianMobile(get("celular")); err != nil {
		fail("el celular no es un celular de Colombia (10 dígitos que empiezan por 3)")
	} else {
		row.Phone = phone
	}
	kind, ok := importKinds[csvx.Fold(get("tipo"))]
	if !ok {
		fail("el tipo de vehículo debe ser carro o moto")
	}
	plate, err := ParsePlate(get("placa"))
	if err != nil {
		fail("la placa no es válida")
		return row
	}
	row.Input = NewVehicleInput{Plate: plate, Kind: string(kind), ConfirmSpecial: true}
	if pk, known := plate.Kind(); ok && known && pk != kind {
		fail("la placa " + plate.String() + " es de " + string(pk) + ", no de " + string(kind))
		return row
	}
	row.Input.Details, row.Input.Engine = importSheet(get, fail)
	if !ok || len(row.Errors) > 0 {
		return row
	}
	// Las mismas reglas del alta en el mostrador (HU-10, HU-11): marca, año, cilindraje.
	if _, err := NewVehicle(shared.NewID(), shared.NewID(), row.Input, now); err != nil {
		var e *shared.Error
		if errors.As(err, &e) {
			fail(strings.TrimSuffix(lowerFirst(e.Message), "."))
		} else {
			fail("el vehículo no es válido")
		}
	}
	return row
}

// importSheet lee la ficha opcional. Lo que falte (cilindraje, turbo) deja el motor pendiente (HU-11, criterio 4).
func importSheet(get func(string) string, fail func(string)) (Details, Engine) {
	var d Details
	var e Engine
	text := func(col string) *string {
		if v := strings.Join(strings.Fields(get(col)), " "); v != "" {
			return &v
		}
		return nil
	}
	num := func(col, msg string) *int {
		v := strings.NewReplacer(".", "", " ", "").Replace(get(col))
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			fail(msg)
			return nil
		}
		return &n
	}
	d.Brand, d.Line = text("marca"), text("linea")
	d.Year = num("modelo", "el modelo debe ser un año, como 2015")
	d.Mileage = num("kilometraje", "el kilometraje debe ser un número")
	e.DisplacementCC = num("cilindraje", "el cilindraje debe ser un número, como 1600")
	switch csvx.Fold(get("turbo")) {
	case "":
	case "si", "s", "x", "turbo":
		t := true
		e.Turbo = &t
	case "no", "n", "atmosferico":
		t := false
		e.Turbo = &t
	default:
		fail("en turbo escribe sí o no")
	}
	// Energía y caja con las mismas claves de la ficha (gasolina, diesel…; manual, automatica…): NewVehicle las valida.
	if v := strings.ReplaceAll(csvx.Fold(get("energia")), " ", "_"); v != "" {
		e.Energy = &v
	}
	if v := strings.ReplaceAll(csvx.Fold(get("caja")), " ", "_"); v != "" {
		e.Transmission = &v
	}
	return d, e
}

// markImportDuplicates marca la placa repetida en el archivo y el celular que aparece con dos nombres distintos.
func markImportDuplicates(rows []ImportRow) {
	plates := map[string]int{}
	names := map[string]ImportRow{}
	for i := range rows {
		r := &rows[i]
		if p := r.Input.Plate.String(); p != "" {
			if first, ok := plates[p]; ok {
				r.Errors = append(r.Errors, "la placa "+p+" ya está en la fila "+strconv.Itoa(first))
			} else {
				plates[p] = r.Line
			}
		}
		if r.Phone == "" {
			continue
		}
		if first, ok := names[r.Phone]; ok && csvx.Fold(first.Name) != csvx.Fold(r.Name) {
			r.Errors = append(r.Errors, "el celular ya está en la fila "+strconv.Itoa(first.Line)+" con otro nombre ("+first.Name+")")
		} else if !ok {
			names[r.Phone] = *r
		}
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToLower(string(r[0])) + string(r[1:])
}
