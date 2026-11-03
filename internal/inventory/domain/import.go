package domain

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strconv"
	"strings"

	"sweveh/internal/shared"
)

// Límites de la importación (HU-37, Propuesta): suficientes para un taller pequeño y acotan el trabajo del servidor.
const (
	MaxImportRows  = 1000
	MaxImportBytes = 1 << 20
)

var (
	ErrImportEmpty   = shared.Validation("importacion_vacia", "El archivo no tiene filas. Usa la plantilla y llena al menos una fila.").WithField("archivo")
	ErrImportHeader  = shared.Validation("importacion_columnas", "Faltan columnas obligatorias: referencia, nombre y precio.").WithField("archivo")
	ErrImportTooMany = shared.Validation("importacion_grande", "El archivo tiene más de 1.000 filas o pesa más de 1 MB. Divídelo en partes.").WithField("archivo")
	ErrImportFormat  = shared.Validation("importacion_formato", "No pudimos leer el archivo. Guárdalo como CSV (separado por comas o punto y coma).").WithField("archivo")
)

// ImportColumns es el orden de la plantilla. Las obligatorias son referencia, nombre y precio.
var ImportColumns = []string{"referencia", "nombre", "tipo", "marca", "categoria", "ubicacion", "referencia_oem", "costo", "precio", "existencia", "minimo"}

// ImportRow es una fila del archivo ya validada: Errors vacío = se puede importar.
type ImportRow struct {
	Line   int // número de fila en el archivo, contando el encabezado como 1
	Part   Part
	Errors []string
}

// ParseImport lee un CSV de la plantilla y valida cada fila con las reglas del repuesto (HU-37).
// No importa nada: el caso de uso decide si confirma. Acepta "," o ";" (Excel en español usa ";") y BOM.
func ParseImport(data []byte) ([]ImportRow, error) {
	if len(data) > MaxImportBytes {
		return nil, ErrImportTooMany
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectSeparator(data)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return nil, ErrImportEmpty
	}
	if err != nil {
		return nil, ErrImportFormat
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range []string{"referencia", "nombre", "precio"} {
		if _, ok := idx[need]; !ok {
			return nil, ErrImportHeader
		}
	}
	var rows []ImportRow
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ErrImportFormat
		}
		if blank(rec) {
			continue
		}
		if len(rows) == MaxImportRows {
			return nil, ErrImportTooMany
		}
		rows = append(rows, parseRow(line, rec, idx))
	}
	if len(rows) == 0 {
		return nil, ErrImportEmpty
	}
	markDuplicates(rows)
	return rows, nil
}

func detectSeparator(data []byte) rune {
	first, _, _ := bytes.Cut(data, []byte("\n"))
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		return ';'
	}
	return ','
}

func blank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func parseRow(line int, rec []string, idx map[string]int) ImportRow {
	get := func(col string) string {
		if i, ok := idx[col]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	row := ImportRow{Line: line}
	num := func(col string, dst *int64) {
		v := strings.NewReplacer(".", "", "$", "", " ", "").Replace(get(col)) // "65.000" y "$ 65.000" son pesos
		if v == "" {
			return
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			row.Errors = append(row.Errors, col+": escribe un número entero sin decimales")
			return
		}
		*dst = n
	}
	var cost, price, stock, minimum int64
	num("costo", &cost)
	num("precio", &price)
	num("existencia", &stock)
	num("minimo", &minimum)
	if get("precio") == "" {
		row.Errors = append(row.Errors, "precio: es obligatorio")
	}
	row.Part = Part{Ref: get("referencia"), Name: get("nombre"), Kind: strings.ToLower(get("tipo")), Brand: get("marca"),
		Category: get("categoria"), Location: get("ubicacion"), OEM: get("referencia_oem"),
		Cost: cost, Price: price, Stock: int(stock), Min: int(minimum)}
	if err := row.Part.Normalize(); err != nil {
		var se *shared.Error
		if errors.As(err, &se) {
			row.Errors = append(row.Errors, se.Message)
		} else {
			row.Errors = append(row.Errors, err.Error())
		}
	}
	return row
}

// markDuplicates marca la referencia repetida dentro del mismo archivo (la primera se queda).
func markDuplicates(rows []ImportRow) {
	seen := map[string]int{}
	for i := range rows {
		ref := rows[i].Part.Ref
		if ref == "" {
			continue
		}
		if first, ok := seen[ref]; ok {
			rows[i].Errors = append(rows[i].Errors, "referencia repetida en la fila "+strconv.Itoa(first))
			continue
		}
		seen[ref] = rows[i].Line
	}
}
