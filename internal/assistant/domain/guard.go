package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Guardias del asesor (ADR-0029): la IA redacta, pero lo que sale al cliente cumple esto o no sale.

// MaxReply es el largo máximo de una respuesta del asesor: WhatsApp se lee en el celular.
const MaxReply = 600

var (
	ErrReplyEmpty = errors.New("respuesta vacía")
	ErrReplyLong  = errors.New("respuesta demasiado larga")
	ErrReplyLink  = errors.New("la respuesta trae un enlace")
	ErrReplyPrice = errors.New("precio que no salió de una consulta")
)

var (
	reBold    = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	reHeading = regexp.MustCompile(`(?m)^#{1,6}\s*`)
	reBlank   = regexp.MustCompile(`\n{3,}`)
)

// Tidy deja el texto como se escribe en WhatsApp: **negrita** → *negrita*, sin títulos de markdown ni renglones de más.
func Tidy(text string) string {
	t := reBold.ReplaceAllString(text, "*$1*")
	t = reHeading.ReplaceAllString(t, "")
	return strings.TrimSpace(reBlank.ReplaceAllString(t, "\n\n"))
}

var reLink = regexp.MustCompile(`(?i)(https?://|www\.|\b[a-z0-9-]+\.(com|co|net|org|io|app|ly)\b)`)

// Guard revisa la respuesta del asesor: no vacía, corta, sin enlaces y con cada precio ($) igual a uno que trajo una
// tool en esta conversación (quoted). Así la IA no puede inventar un precio aunque redacte el texto (RF-17).
func Guard(text string, quoted map[int64]bool) error {
	t := strings.TrimSpace(text)
	switch {
	case t == "":
		return ErrReplyEmpty
	case len([]rune(t)) > MaxReply:
		return ErrReplyLong
	case reLink.MatchString(t):
		return ErrReplyLink
	}
	for _, m := range reMoney.FindAllStringSubmatch(t, -1) {
		var n int64
		_, _ = fmt.Sscanf(strings.ReplaceAll(m[1], ".", ""), "%d", &n)
		if !quoted[n] {
			return fmt.Errorf("%w: %s", ErrReplyPrice, Money(n))
		}
	}
	return nil
}
