package shared

import "strings"

// ErrPhoneInvalid: el celular no es un móvil colombiano de 10 dígitos (docs/08 §4).
var ErrPhoneInvalid = Validation("celular_invalido",
	"Escribe un celular colombiano de 10 dígitos que empiece por 3, por ejemplo 3001234567.")

// ParseColombianMobile normaliza un celular a E.164 (+573XXXXXXXXX).
// Acepta espacios, guiones, puntos y paréntesis, y el indicativo +57 o 57 delante.
func ParseColombianMobile(raw string) (string, error) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')':
		case r == '+' && b.Len() == 0:
		default:
			return "", ErrPhoneInvalid
		}
	}
	digits := b.String()
	if len(digits) == 12 && strings.HasPrefix(digits, "57") {
		digits = digits[2:]
	}
	if len(digits) != 10 || digits[0] != '3' {
		return "", ErrPhoneInvalid
	}
	return "+57" + digits, nil
}

// NationalNumber devuelve los 10 dígitos de un celular en E.164 colombiano.
func NationalNumber(e164 string) string {
	return strings.TrimPrefix(e164, "+57")
}
