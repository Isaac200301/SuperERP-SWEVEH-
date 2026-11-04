// Package domain contiene la clasificación de mensajes, el armado de borradores con datos del taller
// y su validación (HU-52 a HU-58). Sin I/O. La IA propone, el código valida, el humano aprueba (R-08).
package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Intent es lo que el cliente pide.
type Intent struct {
	Kind     string // ver Intents
	Plate    string
	Wants    []string // palabras clave del servicio pedido
	ForDate  string   // "hoy" | "manana" | ""
	Services []string // nombres exactos de servicios o repuestos de la lista del taller
	Others   []string // lo pedido que no está en la lista
	Vehicle  Vehicle
	Symptom  string
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
	case has(t, "que tiene mi", "que tiene la", "que tiene el", "que le pasa", "que sera", "diagnost"):
		in.Kind = "diagnostico"
	case has(t, "ya esta", "esta lista", "esta listo", "ya puedo ir", "ya puedo pasar", "puedo recoger", "como va", "como vamos", "ya quedo", "cuando esta", "cuando me lo", "estado de"):
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
	in.Vehicle = VehicleIn(t)
	return in
}

// Interpret es el intérprete por reglas (proveedor "reglas"): lo mismo que se le pide a la IA, sin costo y
// determinista. Los servicios salen de la lista del taller por coincidencia de palabras (Matches).
func Interpret(text string, catalog []string) Understanding {
	in := Classify(text)
	u := Understanding{Intent: in.Kind, Vehicle: in.Vehicle, Plate: in.Plate, ForDate: in.ForDate, Services: []string{}, Others: []string{}}
	if in.Kind != "humano" && in.Kind != "fuera_tema" && in.Kind != "saludo" && in.Kind != "estado" {
		u.Symptom = strings.TrimSpace(text)
		if len([]rune(u.Symptom)) > 300 {
			u.Symptom = string([]rune(u.Symptom)[:300])
		}
		for _, c := range catalog {
			if Matches(c, in.Wants) {
				u.Services = append(u.Services, c)
			}
		}
	}
	return u
}

// Modelos frecuentes en Colombia → marca y tipo (para reglas; la IA no depende de esta lista).
var models = []struct{ word, brand, kind string }{
	{"spark", "Chevrolet", "carro"}, {"aveo", "Chevrolet", "carro"}, {"sail", "Chevrolet", "carro"}, {"onix", "Chevrolet", "carro"},
	{"logan", "Renault", "carro"}, {"sandero", "Renault", "carro"}, {"duster", "Renault", "carro"}, {"twingo", "Renault", "carro"},
	{"picanto", "Kia", "carro"}, {"sportage", "Kia", "carro"}, {"mazda", "Mazda", "carro"}, {"corolla", "Toyota", "carro"},
	{"hilux", "Toyota", "carro"}, {"fortuner", "Toyota", "carro"}, {"swift", "Suzuki", "carro"}, {"gol", "Volkswagen", "carro"},
	{"nmax", "Yamaha", "moto"}, {"n-max", "Yamaha", "moto"}, {"bws", "Yamaha", "moto"}, {"biwis", "Yamaha", "moto"}, {"xtz", "Yamaha", "moto"},
	{"crypton", "Yamaha", "moto"}, {"libero", "Yamaha", "moto"}, {"ybr", "Yamaha", "moto"}, {"fz", "Yamaha", "moto"},
	{"pulsar", "Bajaj", "moto"}, {"boxer", "Bajaj", "moto"}, {"discover", "Bajaj", "moto"}, {"apache", "TVS", "moto"},
	{"akt", "AKT", "moto"}, {"gixxer", "Suzuki", "moto"}, {"duke", "KTM", "moto"}, {"xr", "Honda", "moto"}, {"cb", "Honda", "moto"},
}

var lineSuffix = map[string]bool{"gt": true, "lt": true, "ls": true, "life": true, "sport": true, "ns": true, "rs": true,
	"110": true, "125": true, "150": true, "160": true, "180": true, "190": true, "200": true, "250": true}

var reYear = regexp.MustCompile(`\b(19[5-9][0-9]|20[0-4][0-9])\b`)

// VehicleIn saca tipo, marca, línea y año de un texto ya normalizado (HU-52).
func VehicleIn(t string) Vehicle {
	var v Vehicle
	words := strings.FieldsFunc(t, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' })
	for i, w := range words {
		for _, m := range models {
			if w != m.word {
				continue
			}
			v.Brand, v.Type, v.Line = m.brand, m.kind, Capitalize(w)
			if strings.ToLower(m.brand) == w {
				v.Line = ""
			}
			if i+1 < len(words) && lineSuffix[words[i+1]] {
				v.Line = strings.TrimSpace(v.Line + " " + strings.ToUpper(words[i+1]))
			}
			break
		}
		if v.Brand != "" {
			break
		}
	}
	switch {
	case has(t, "moto", "la nave", "scooter", "motico"):
		v.Type = "moto"
	case v.Type == "" && has(t, "carro", "camioneta", "campero", "automovil"):
		v.Type = "carro"
	}
	if m := reYear.FindString(t); m != "" {
		_, _ = fmt.Sscanf(m, "%d", &v.Year)
	}
	return v
}

var stop = map[string]bool{"cambio": true, "cambiar": true, "cuanto": true, "cobran": true, "puedo": true, "llevarlo": true,
	"manana": true, "temprano": true, "cuando": true, "buenas": true, "hola": true, "esta": true, "para": true, "por": true,
	"carro": true, "moto": true, "precio": true, "delanteras": false}

// stemAlias junta raíces del mismo servicio ("frenar" → "freno", "llantas" → "llant").
var stemAlias = map[string]string{"frena": "freno", "frene": "freno", "pasta": "pasti"}

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
		if a, ok := stemAlias[s]; ok {
			s = a
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
