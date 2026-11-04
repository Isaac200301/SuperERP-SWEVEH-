package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Priced es un servicio o repuesto del taller con su precio (nil = sin precio cargado).
type Priced struct {
	Name    string
	Price   *int64
	Kind    string // servicio | repuesto
	Vehicle string // carro | moto | ambos ("" = ambos)
}

// CatalogItem es lo que ve el intérprete de la lista del taller: nombre y tipo de vehículo, sin precios (la IA
// nunca los necesita: el precio lo pone el código, HU-53).
type CatalogItem struct {
	Name    string `json:"nombre"`
	Vehicle string `json:"tipo"`
}

// Items junta servicios y repuestos sin repetir nombres; un nombre de carro y de moto queda como "ambos".
func Items(lists ...[]Priced) []CatalogItem {
	var out []CatalogItem
	idx := map[string]int{}
	for _, l := range lists {
		for _, p := range l {
			v := p.Vehicle
			if v == "" {
				v = "ambos"
			}
			if i, ok := idx[p.Name]; ok {
				if out[i].Vehicle != v {
					out[i].Vehicle = "ambos"
				}
				continue
			}
			idx[p.Name] = len(out)
			out = append(out, CatalogItem{Name: p.Name, Vehicle: v})
		}
	}
	return out
}

// Names son los nombres de la lista cerrada (enum del esquema y validación).
func Names(items []CatalogItem) []string {
	out := make([]string, 0, len(items))
	for _, i := range items {
		out = append(out, i.Name)
	}
	return out
}

// Source dice de dónde salió cada dato del borrador (HU-64, RF-19).
type Source struct {
	Field  string `json:"campo"`
	Value  string `json:"valor"`
	Origin string `json:"origen"`
}

// DraftInput son los datos del taller con los que se arma el borrador (HU-53).
type DraftInput struct {
	CustomerName string
	Workshop     string
	Intent       Intent
	Services     []Priced
	Parts        []Priced
	NextOpening  string // "mañana a las 7:30 a. m." según el horario (HU-08)
	HasPlate     bool
	Attends      string // carro | moto | ambos: lo que atiende el taller (perfil, HU-03)
}

// Draft es la propuesta para el dueño.
type Draft struct {
	Text    string
	Sources []Source
}

// Money da el formato COP sin decimales.
func Money(v int64) string {
	s := fmt.Sprintf("%d", v)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return "$" + b.String()
}

// BuildDraft arma el borrador SOLO con precios, servicios y horario cargados por el taller (HU-53).
// Si un servicio no tiene precio dice "el taller te confirma el valor" (HU-07, criterio 2). Nunca inventa: lo que
// el cliente pidió y no está en la lista queda como "otro" en las fuentes para el dueño (HU-52, criterio 4).
func BuildDraft(in DraftInput) Draft {
	var parts []string
	var src []Source
	greet := Greet("¡Hola", in.CustomerName)
	switch {
	case in.Intent.Kind == "saludo":
		return Draft{Text: fmt.Sprintf("%s Te escribe el asistente de %s. Puedo darte precios de nuestros servicios, agendar tu visita o "+
			"contarte cómo va tu vehículo. ¿En qué te ayudo?", greet, in.Workshop)}
	case in.Intent.Kind == "otro" && len(in.Intent.Services)+len(in.Intent.Others) == 0:
		return Draft{Text: "Perdona, no te entendí bien. Puedo ayudarte con precios de servicios, citas o el estado de tu vehículo. ¿Qué necesitas?"}
	}
	vt := in.Intent.Vehicle.Type
	if vt != "" && (in.Attends == "carro" || in.Attends == "moto") && in.Attends != vt { // HU-52, criterio 3
		return Draft{Text: fmt.Sprintf("%s Gracias por escribirnos. Por ahora el taller solo atiende %ss, así que no podemos ayudarte con tu %s.",
			greet, in.Attends, vt), Sources: []Source{{Field: "atiende", Value: in.Attends + "s", Origin: "Perfil del taller"}}}
	}
	fits := func(p Priced) bool { return vt == "" || p.Vehicle == "" || p.Vehicle == "ambos" || p.Vehicle == vt }
	label := func(p Priced) string {
		if vt == "" && (p.Vehicle == "carro" || p.Vehicle == "moto") {
			return p.Name + " (" + p.Vehicle + ")"
		}
		return p.Name
	}
	wanted := map[string]bool{}
	for _, n := range in.Intent.Services {
		wanted[Normalize(n)] = true
	}
	if in.Intent.Kind == "diagnostico" { // HU-58, criterio 2: no se diagnostica; se ofrece la revisión de lista
		offer := "Sin revisar el vehículo no te podemos decir qué tiene; el taller te confirma el valor de la revisión."
		for _, s := range in.Services {
			n := Normalize(s.Name)
			if fits(s) && (strings.Contains(n, "revis") || strings.Contains(n, "diagn") || strings.Contains(n, "escan")) {
				wanted[n] = true
				offer = "Sin revisar el vehículo no te podemos decir qué tiene; te ofrecemos la revisión."
				break
			}
		}
		parts = append(parts, offer)
	}
	var matched []string
	for _, s := range in.Services {
		if !fits(s) || !wanted[Normalize(s.Name)] {
			continue
		}
		if s.Price != nil {
			matched = append(matched, fmt.Sprintf("%s cuesta %s de mano de obra", Lower(label(s)), Money(*s.Price)))
			src = append(src, Source{Field: "precio", Value: Money(*s.Price), Origin: "Servicio: " + label(s)})
		} else {
			matched = append(matched, fmt.Sprintf("para %s el taller te confirma el valor", Lower(label(s))))
			src = append(src, Source{Field: "precio", Value: "sin precio", Origin: "Servicio: " + label(s)})
		}
	}
	for _, p := range in.Parts {
		if !fits(p) || !wanted[Normalize(p.Name)] || p.Price == nil {
			continue
		}
		matched = append(matched, fmt.Sprintf("el repuesto %s vale %s", p.Name, Money(*p.Price)))
		src = append(src, Source{Field: "precio", Value: Money(*p.Price), Origin: "Inventario: " + p.Name})
	}
	if len(matched) > 0 {
		parts = append(parts, Capitalize(strings.Join(matched, "; "))+".")
	}
	for _, o := range in.Intent.Others {
		src = append(src, Source{Field: "otro", Value: o, Origin: "No está en la lista del taller"})
	}
	if len(in.Intent.Others) > 0 || (len(matched) == 0 && in.Intent.Kind == "cotizar") {
		parts = append(parts, "Para darte el valor exacto el taller necesita revisar el vehículo; el taller te confirma el valor.")
	}
	if (in.Intent.ForDate != "" || in.Intent.Kind == "cita") && in.NextOpening != "" {
		parts = append(parts, "Tenemos cupo "+strings.TrimSuffix(in.NextOpening, ".")+".")
		src = append(src, Source{Field: "horario", Value: in.NextOpening, Origin: "Horario del taller"})
	}
	if !in.HasPlate && (in.Intent.Kind == "cotizar" || in.Intent.Kind == "cita" || in.Intent.Kind == "diagnostico") {
		parts = append(parts, "¿Me regalas la placa del vehículo?")
	}
	if len(parts) == 0 {
		parts = append(parts, "Cuéntanos qué necesita tu vehículo y te ayudamos.")
	}
	return Draft{Text: greet + " " + strings.Join(parts, " "), Sources: src}
}

var reMoney = regexp.MustCompile(`\$\s?([0-9]{1,3}(?:\.[0-9]{3})+|[0-9]+)`)

// Validate revisa todo lo que sale hacia el cliente (HU-54, RF-17): ningún precio fuera de la lista del taller.
// Lo que no valida se descarta y pasa a "Lo contesto yo".
func Validate(text string, allowed []Priced) error {
	ok := map[string]bool{}
	for _, a := range allowed {
		if a.Price != nil {
			ok[Money(*a.Price)] = true
		}
	}
	for _, m := range reMoney.FindAllStringSubmatch(text, -1) {
		v := "$" + m[1]
		if !strings.Contains(m[1], ".") && len(m[1]) > 3 {
			var n int64
			_, _ = fmt.Sscanf(m[1], "%d", &n)
			v = Money(n)
		}
		if !ok[v] {
			return fmt.Errorf("precio %s no está en la lista del taller", v)
		}
	}
	if strings.TrimSpace(text) == "" || len([]rune(text)) > 1000 {
		return fmt.Errorf("borrador vacío o demasiado largo")
	}
	return nil
}

func Lower(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToLower(string(r[0])) + string(r[1:])
}

func Capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// FirstName es el primer nombre del cliente, o "" si es un nombre de relleno ("Cliente WhatsApp", "Cliente de
// prueba", una sola letra o números): así no se saluda a nadie como "Cliente".
func FirstName(name string) string {
	f := strings.Fields(name)
	if len(f) == 0 {
		return ""
	}
	first := f[0]
	low := strings.ToLower(first)
	if len([]rune(first)) < 2 || low == "cliente" || strings.Trim(first, "0123456789+") == "" {
		return ""
	}
	return first
}

// Greet arma "¡Hola Laura!" o "¡Hola!" si no hay un nombre real. prefix va sin el cierre: "¡Hola", "¡Gracias".
func Greet(prefix, name string) string {
	if n := FirstName(name); n != "" {
		return prefix + " " + n + "!"
	}
	return prefix + "!"
}
