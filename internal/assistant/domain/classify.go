// Package domain contiene la clasificación de mensajes, el armado de borradores con datos del taller
// y su validación (HU-52 a HU-58). Sin I/O. La IA propone, el código valida, el humano aprueba (R-08).
package domain

import (
	"regexp"
	"strings"
	"unicode"
)

// Intent es lo que el cliente pide.
type Intent struct {
	Kind    string // estado | cotizar | cita | humano | fuera_tema | saludo | otro
	Plate   string
	Wants   []string // palabras clave del servicio pedido
	ForDate string   // "hoy" | "manana" | ""
}

// Normalize pasa a minúsculas y quita tildes para comparar.
func Normalize(s string) string {
	r := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
	return r.Replace(strings.ToLower(s))
}

var rePlate = regexp.MustCompile(`\b([A-Za-z]{3})[\s-]?([0-9]{2})([0-9A-Za-z]?)\b`)

// FindPlate busca una placa de carro (ABC123) o moto (ABC12D, ABC12) en el texto.
func FindPlate(text string) string {
	m := rePlate.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1] + m[2] + m[3])
}

func has(t string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// Workshop words: si el mensaje no toca nada del taller y sí un tema ajeno, es fuera de tema (HU-58).
var offTopic = []string{"receta", "chiste", "clima", "futbol", "partido", "tarea", "poema", "programa en", "codigo", "bitcoin", "horoscopo", "politica"}
var onTopic = []string{"carro", "moto", "vehiculo", "freno", "aceite", "motor", "llanta", "taller", "placa", "revision", "cambio", "servicio", "cita", "precio", "cuanto", "repar", "suena", "chilla", "ruido", "bateria", "luz", "listo"}

// Classify clasifica con reglas locales (proveedor "reglas": sin costo, determinista).
// El texto del cliente es dato, nunca instrucción (R-08): no se interpreta como orden al sistema.
func Classify(text string) Intent {
	t := Normalize(text)
	in := Intent{Kind: "otro", Plate: FindPlate(text)}
	switch {
	case has(t, "hablar con", "una persona", "asesor", "humano", "alguien que me"):
		in.Kind = "humano"
	case has(t, "ya esta", "esta listo", "como va", "como vamos", "ya quedo", "cuando esta", "cuando me lo", "estado de"):
		in.Kind = "estado"
	case has(t, "cuanto", "precio", "cobran", "vale", "valor", "cotiz", "costo"):
		in.Kind = "cotizar"
	case has(t, "cita", "agendar", "turno", "puedo llevar", "llevarlo", "llevarla", "cupo"):
		in.Kind = "cita"
	case has(t, offTopic...) && !has(t, onTopic...):
		in.Kind = "fuera_tema"
	case has(t, "hola", "buenas", "buenos dias", "buenas tardes") && len(strings.Fields(t)) <= 4:
		in.Kind = "saludo"
	}
	if in.Kind != "humano" && in.Kind != "fuera_tema" && in.Kind != "estado" && has(t, "manana", "hoy", "cupo", "llevar") {
		if has(t, "manana") {
			in.ForDate = "manana"
		} else if has(t, "hoy") {
			in.ForDate = "hoy"
		}
	}
	in.Wants = Stems(t)
	return in
}

var stop = map[string]bool{"cambio": true, "cambiar": true, "cuanto": true, "cobran": true, "puedo": true, "llevarlo": true,
	"manana": true, "temprano": true, "cuando": true, "buenas": true, "hola": true, "esta": true, "para": true, "por": true,
	"carro": true, "moto": true, "precio": true, "delanteras": false}

// Stems devuelve raíces de 5 letras de las palabras útiles ("pastillas" → "pasti", "freno" → "freno").
func Stems(t string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(Normalize(t), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) < 4 || stop[w] {
			continue
		}
		s := w
		if len(s) > 5 {
			s = s[:5]
		}
		dup := false
		for _, o := range out {
			dup = dup || o == s
		}
		if !dup {
			out = append(out, s)
		}
	}
	return out
}

// Matches indica si un nombre (servicio o repuesto) coincide con lo pedido. Se ignora "revis"/"cambi" sola.
func Matches(name string, wants []string) bool {
	ns := Stems(name)
	for _, w := range wants {
		if w == "revis" || w == "cambi" || w == "servi" {
			continue
		}
		for _, n := range ns {
			if n == w {
				return true
			}
		}
	}
	return false
}
