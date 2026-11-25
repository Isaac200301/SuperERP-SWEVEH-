package pdfx

import "strings"

// Anchos de Helvetica y Helvetica-Bold (milésimas de em) para ASCII 32 a 126, de las tablas AFM estándar de Adobe.
// Sirven para alinear a la derecha y partir líneas; no hace falta más precisión para una cotización.
var (
	regular = [95]int{
		278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, // espacio a /
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556, // 0 a 9
		278, 278, 584, 584, 584, 556, 1015, // : ; < = > ? @
		667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, // A a Z
		278, 278, 278, 469, 556, 333, // [ \ ] ^ _ `
		556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, // a a z
		334, 260, 334, 584, // { | } ~
	}
	bold = [95]int{
		278, 333, 474, 556, 556, 889, 722, 238, 333, 333, 389, 584, 278, 333, 278, 278,
		556, 556, 556, 556, 556, 556, 556, 556, 556, 556,
		333, 333, 584, 584, 584, 611, 975,
		722, 722, 722, 722, 667, 611, 778, 722, 278, 556, 722, 611, 833, 722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611,
		333, 278, 333, 584, 556, 333,
		556, 611, 556, 611, 556, 333, 611, 611, 278, 278, 556, 278, 889, 611, 611, 611, 611, 389, 556, 333, 611, 556, 778, 556, 556, 500,
		389, 280, 389, 584,
	}
)

// baseLetter lleva una letra con tilde a la letra sin ella: mide lo mismo.
var baseLetter = func() map[rune]rune {
	m := map[rune]rune{}
	for base, accented := range map[rune]string{
		'A': "ÀÁÂÃÄÅ", 'C': "Ç", 'E': "ÈÉÊË", 'I': "ÌÍÎÏ", 'N': "Ñ", 'O': "ÒÓÔÕÖ", 'U': "ÙÚÛÜ", 'Y': "Ý",
		'a': "àáâãäå", 'c': "ç", 'e': "èéêë", 'i': "ìíîï", 'n': "ñ", 'o': "òóôõö", 'u': "ùúûü", 'y': "ýÿ",
	} {
		for _, r := range accented {
			m[r] = base
		}
	}
	return m
}()

func runeWidth(r rune, isBold bool) int {
	tbl := &regular
	if isBold {
		tbl = &bold
	}
	if base, ok := baseLetter[r]; ok {
		r = base
	}
	if r >= 32 && r <= 126 {
		return tbl[r-32]
	}
	return 556 // el resto (símbolos, guiones largos) mide como un dígito
}

// Width es lo que ocupa el texto, en puntos, con ese tamaño de letra.
func Width(s string, size float64, isBold bool) float64 {
	total := 0
	for _, r := range s {
		total += runeWidth(r, isBold)
	}
	return float64(total) * size / 1000
}

// cutWord corta de w la parte que cabe en maxWidth (al menos una letra, sin partir un carácter de varios bytes).
func cutWord(w string, size float64, isBold bool, maxWidth float64) (head, rest string) {
	cut := 1
	for cut < len(w) && Width(w[:cut+1], size, isBold) <= maxWidth {
		cut++
	}
	for cut < len(w) && w[cut]&0xC0 == 0x80 {
		cut++
	}
	return w[:cut], w[cut:]
}

// Wrap parte el texto en líneas que no pasen de maxWidth; una palabra más larga que la línea se corta por letras.
func Wrap(s string, size float64, isBold bool, maxWidth float64) []string {
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for _, w := range strings.Fields(s) {
		for Width(w, size, isBold) > maxWidth { // palabra que no cabe sola
			var head string
			head, w = cutWord(w, size, isBold, maxWidth)
			flush()
			lines = append(lines, head)
		}
		if w == "" {
			continue
		}
		if next := strings.TrimSpace(cur + " " + w); cur == "" || Width(next, size, isBold) <= maxWidth {
			cur = next
			continue
		}
		flush()
		cur = w
	}
	flush()
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
