// Package csvx lee los CSV que suben los talleres (plantillas de importación, HU-37 y HU-92): acepta "," o ";"
// (Excel en español usa ";"), BOM y archivos de Excel en Windows (no UTF-8). Cada módulo valida sus columnas.
package csvx

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

var (
	ErrTooLarge = errors.New("csvx: archivo o filas por encima del límite")
	ErrEmpty    = errors.New("csvx: sin filas")
	ErrFormat   = errors.New("csvx: no es un CSV legible")
)

// Row es una fila con su número en el archivo, contando el encabezado como 1.
type Row struct {
	Line   int
	Fields []string
}

// Table es el archivo leído: el encabezado y las filas que no están en blanco.
type Table struct {
	Header []string
	Rows   []Row
}

// Read lee el archivo con los límites de cada importación. No valida columnas.
func Read(data []byte, maxBytes, maxRows int) (Table, error) {
	if len(data) > maxBytes {
		return Table{}, ErrTooLarge
	}
	data = ToUTF8(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}))
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = separator(data)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if errors.Is(err, io.EOF) {
		return Table{}, ErrEmpty
	}
	if err != nil {
		return Table{}, ErrFormat
	}
	t := Table{Header: header}
	for line := 2; ; line++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Table{}, ErrFormat
		}
		if blank(rec) {
			continue
		}
		if len(t.Rows) == maxRows {
			return Table{}, ErrTooLarge
		}
		t.Rows = append(t.Rows, Row{Line: line, Fields: rec})
	}
	if len(t.Rows) == 0 {
		return Table{}, ErrEmpty
	}
	return t, nil
}

// Columns traduce el encabezado con los alias de la plantilla (ya pasados por Fold) a la posición de cada columna.
// Devuelve avisos por columnas que no se reconocen o se repiten; no son errores.
func Columns(header []string, aliases map[string]string) (map[string]int, []string) {
	idx := map[string]int{}
	var warnings []string
	for i, h := range header {
		col, ok := aliases[Fold(h)]
		_, dup := idx[col]
		switch {
		case strings.TrimSpace(h) == "":
			continue
		case !ok:
			warnings = append(warnings, "La columna \""+strings.TrimSpace(h)+"\" no la reconocemos y se ignora.")
		case dup:
			warnings = append(warnings, "La columna \""+strings.TrimSpace(h)+"\" está repetida; se usa la primera.")
		default:
			idx[col] = i
		}
	}
	return idx, warnings
}

// Cell es el valor de la columna en la fila, sin espacios; "" si la columna no está.
func Cell(rec []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

// Fold deja un texto comparable: minúsculas, sin tildes, sin lo que va entre paréntesis y con espacios simples.
func Fold(s string) string {
	if i := strings.Index(s, "("); i >= 0 {
		s = s[:i]
	}
	s = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
		"Á", "a", "É", "e", "Í", "i", "Ó", "o", "Ú", "u", "Ü", "u", "Ñ", "n").Replace(strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

// ToUTF8 convierte un archivo de Excel en Windows (cp1252) a UTF-8; si ya es UTF-8 no lo toca.
func ToUTF8(b []byte) []byte {
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

func separator(data []byte) rune {
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
