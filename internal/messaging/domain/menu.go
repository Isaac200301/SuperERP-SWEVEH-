package domain

import (
	"strconv"
	"strings"
	"unicode"
)

// Menu escribe las opciones como texto numerado, para canales que no pueden mandar botones (WAHA, ADR-0027).
// La respuesta del cliente se lee con MatchReply.
func Menu(text string, b []Button) string {
	if len(b) == 0 {
		return text
	}
	var s strings.Builder
	s.WriteString(strings.TrimRight(text, "\n "))
	s.WriteString("\n\nResponde con el número:")
	for i, x := range b {
		s.WriteString("\n" + strconv.Itoa(i+1) + ". " + x.Title)
	}
	return s.String()
}

// MatchReply reconoce la opción que el cliente eligió escribiendo su número ("1", "2.") o su título
// ("acepto", "No acepto"), sin importar tildes, mayúsculas, signos ni emojis. Cualquier otra cosa no es una
// opción y sigue como texto normal.
func MatchReply(reply string, b []Button) (string, bool) {
	r := fold(reply)
	if r == "" {
		return "", false
	}
	if n, err := strconv.Atoi(r); err == nil {
		if n >= 1 && n <= len(b) {
			return b[n-1].ID, true
		}
		return "", false
	}
	for _, x := range b {
		if fold(x.Title) == r {
			return x.ID, true
		}
	}
	return "", false
}

// fold deja letras sin tilde en minúscula, dígitos y un espacio entre palabras.
func fold(s string) string {
	s = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n").Replace(strings.ToLower(s))
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ")
}
