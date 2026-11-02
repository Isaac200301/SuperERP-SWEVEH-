package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Priced es un servicio o repuesto del taller con su precio (nil = sin precio cargado).
type Priced struct {
	Name  string
	Price *int64
	Kind  string // servicio | repuesto
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

// BuildDraft arma el borrador SOLO con precios, servicios y horario cargados por el taller.
// Si un servicio no tiene precio dice "el taller te confirma el valor" (HU-07, criterio 2). Nunca inventa.
func BuildDraft(in DraftInput) Draft {
	var parts []string
	var src []Source
	greet := "¡Hola!"
	if in.CustomerName != "" {
		greet = "¡Hola " + strings.Fields(in.CustomerName)[0] + "!"
	}
	var matched []string
	for _, s := range in.Services {
		if !Matches(s.Name, in.Intent.Wants) {
			continue
		}
		if s.Price != nil {
			matched = append(matched, fmt.Sprintf("%s cuesta %s de mano de obra", Lower(s.Name), Money(*s.Price)))
			src = append(src, Source{Field: "precio", Value: Money(*s.Price), Origin: "Servicio: " + s.Name})
		} else {
			matched = append(matched, fmt.Sprintf("para %s el taller te confirma el valor", Lower(s.Name)))
			src = append(src, Source{Field: "precio", Value: "sin precio", Origin: "Servicio: " + s.Name})
		}
	}
	for _, p := range in.Parts {
		if !Matches(p.Name, in.Intent.Wants) || p.Price == nil {
			continue
		}
		matched = append(matched, fmt.Sprintf("el repuesto %s vale %s", p.Name, Money(*p.Price)))
		src = append(src, Source{Field: "precio", Value: Money(*p.Price), Origin: "Inventario: " + p.Name})
		break
	}
	if len(matched) > 0 {
		parts = append(parts, Capitalize(strings.Join(matched, "; "))+".")
	} else if in.Intent.Kind == "cotizar" {
		parts = append(parts, "Para darte el valor exacto el taller necesita revisar el vehículo; el taller te confirma el valor.")
	}
	if (in.Intent.ForDate != "" || in.Intent.Kind == "cita") && in.NextOpening != "" {
		parts = append(parts, "Tenemos cupo "+strings.TrimSuffix(in.NextOpening, ".")+".")
		src = append(src, Source{Field: "horario", Value: in.NextOpening, Origin: "Horario del taller"})
	}
	if !in.HasPlate {
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
