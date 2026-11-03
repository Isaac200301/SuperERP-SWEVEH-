package domain

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

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

// columnAliases traduce los encabezados que escribe una persona (ya sin tildes, en minúscula y sin lo que va
// entre paréntesis) a la columna interna. Así sirve el Excel exportado desde la pantalla o uno hecho a mano.
var columnAliases = map[string]string{
	"referencia": "referencia", "ref": "referencia", "codigo": "referencia", "sku": "referencia",
	"nombre": "nombre", "descripcion": "nombre", "repuesto": "nombre",
	"tipo": "tipo", "para": "tipo", "tipo de vehiculo": "tipo", "vehiculo": "tipo",
	"marca":     "marca",
	"categoria": "categoria",
	"ubicacion": "ubicacion", "ubicacion en bodega": "ubicacion",
	"referencia_oem": "referencia_oem", "referencia original": "referencia_oem", "referencia oem": "referencia_oem", "oem": "referencia_oem",
	"oem o equivalente": "referencia_oem",
	"costo":             "costo", "costo unitario": "costo",
	"precio": "precio", "precio de venta": "precio", "precio venta": "precio", "precio_venta": "precio",
	"existencia": "existencia", "stock": "existencia", "cantidad": "existencia", "existencias": "existencia",
	"minimo": "minimo", "stock minimo": "minimo", "minimo en bodega": "minimo",
}

// kindAliases y categoryAliases aceptan el texto que muestra la pantalla además de la clave interna.
var kindAliases = map[string]string{"": "", "carro": "carro", "carros": "carro", "moto": "moto", "motos": "moto",
	"ambos": "ambos", "carro y moto": "ambos", "carros y motos": "ambos", "carro/moto": "ambos"}

var categoryAliases = map[string]string{"": "", "sin categoria": "", "caja y transmision": "transmision", "caja": "transmision",
	"filtro": "filtros", "lubricante": "lubricantes", "llanta": "llantas", "otros": "otro", "electricos": "electrico"}

// ImportFile es el archivo ya leído: las filas validadas y los avisos que no impiden importar.
type ImportFile struct {
	Rows     []ImportRow
	Warnings []string // p. ej. columnas que no reconocimos y se ignoraron
}

// ImportRow es una fila del archivo ya validada: Errors vacío = se puede importar.
type ImportRow struct {
	Line   int // número de fila en el archivo, contando el encabezado como 1
	Part   Part
	Errors []string
}

// ParseImport lee un CSV y valida cada fila con las reglas del repuesto (HU-37). No importa nada: el caso de uso
// decide si confirma. Acepta "," o ";" (Excel en español usa ";"), BOM y archivos de Excel en Windows (no UTF-8).
func ParseImport(data []byte) (ImportFile, error) {
	if len(data) > MaxImportBytes {
		return ImportFile{}, ErrImportTooMany
	}
	data = toUTF8(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}))
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectSeparator(data)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return ImportFile{}, ErrImportEmpty
	}
	if err != nil {
		return ImportFile{}, ErrImportFormat
	}
	idx, warnings := mapHeader(header)
	for _, need := range []string{"referencia", "nombre", "precio"} {
		if _, ok := idx[need]; !ok {
			return ImportFile{}, ErrImportHeader
		}
	}
	out := ImportFile{Warnings: warnings}
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ImportFile{}, ErrImportFormat
		}
		if blank(rec) {
			continue
		}
		if len(out.Rows) == MaxImportRows {
			return ImportFile{}, ErrImportTooMany
		}
		out.Rows = append(out.Rows, parseRow(line, rec, idx))
	}
	if len(out.Rows) == 0 {
		return ImportFile{}, ErrImportEmpty
	}
	markDuplicates(out.Rows)
	return out, nil
}

// fold deja un texto comparable: minúsculas, sin tildes, sin lo que va entre paréntesis y con espacios simples.
func fold(s string) string {
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	s = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n").Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

func mapHeader(header []string) (map[string]int, []string) {
	idx := map[string]int{}
	var warnings []string
	for i, h := range header {
		col, ok := columnAliases[fold(h)]
		switch {
		case strings.TrimSpace(h) == "":
			continue
		case !ok:
			warnings = append(warnings, "La columna \""+strings.TrimSpace(h)+"\" no la reconocemos y se ignora.")
		case hasKey(idx, col):
			warnings = append(warnings, "La columna \""+strings.TrimSpace(h)+"\" está repetida; se usa la primera.")
		default:
			idx[col] = i
		}
	}
	return idx, warnings
}

func hasKey(m map[string]int, k string) bool { _, ok := m[k]; return ok }

// toUTF8 convierte desde Windows-1252 (lo que guarda Excel en Windows como "CSV delimitado por comas") si el
// archivo no es UTF-8 válido. Los bytes 0xA0-0xFF coinciden con Latin-1; 0x80-0x9F usan la tabla de Windows.
func toUTF8(b []byte) []byte {
	if utf8.Valid(b) {
		return b
	}
	var out bytes.Buffer
	for _, c := range b {
		switch {
		case c < 0x80:
			out.WriteByte(c)
		case c < 0xA0:
			out.WriteRune(cp1252[c-0x80])
		default:
			out.WriteRune(rune(c))
		}
	}
	return out.Bytes()
}

var cp1252 = [32]rune{'€', '�', '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', '�', 'Ž', '�',
	'�', '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', '�', 'ž', 'Ÿ'}

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

// reThousands: "65.000", "65,000" o "1.250.000" son pesos con separador de miles.
var reThousands = regexp.MustCompile(`^\d{1,3}([.,]\d{3})+$`)

// parseAmount lee un número entero de pesos o unidades; "" = sin dato.
func parseAmount(raw string) (int64, bool, error) {
	v := strings.NewReplacer("$", "", " ", "").Replace(strings.TrimSpace(raw))
	if v == "" {
		return 0, false, nil
	}
	if reThousands.MatchString(v) {
		v = strings.NewReplacer(".", "", ",", "").Replace(v)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, true, errors.New("escribe un número entero sin decimales")
	}
	return n, true, nil
}

func parseRow(line int, rec []string, idx map[string]int) ImportRow {
	get := func(col string) string {
		if i, ok := idx[col]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	row := ImportRow{Line: line}
	amounts := map[string]int64{}
	for _, col := range []string{"costo", "precio", "existencia", "minimo"} {
		n, set, err := parseAmount(get(col))
		if err != nil {
			row.Errors = append(row.Errors, col+": "+err.Error())
		}
		if set && err == nil {
			amounts[col] = n
		}
		if col == "precio" && !set {
			row.Errors = append(row.Errors, "precio: es obligatorio")
		}
	}
	kind, ok := kindAliases[fold(get("tipo"))]
	if !ok {
		row.Errors = append(row.Errors, "para: escribe Carro, Moto o Carro y moto")
	}
	category := fold(get("categoria"))
	if c, ok := categoryAliases[category]; ok {
		category = c
	}
	row.Part = Part{Ref: get("referencia"), Name: get("nombre"), Kind: kind, Brand: get("marca"), Category: category,
		Location: get("ubicacion"), OEM: get("referencia_oem"), Cost: amounts["costo"], Price: amounts["precio"],
		Stock: int(amounts["existencia"]), Min: int(amounts["minimo"])}
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
