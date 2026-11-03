// Package domain contiene los servicios del taller y su precarga (HU-04, HU-07). Sin I/O.
package domain

import (
	"strings"

	"sweveh/internal/shared"
)

var (
	ErrNotFound = shared.NotFound("servicio_no_encontrado", "No encontramos ese servicio en tu taller.")
	ErrName     = shared.Validation("servicio_nombre", "El servicio necesita un nombre de hasta 120 letras.").WithField("nombre")
	ErrKind     = shared.Validation("servicio_tipo", "Indica si el servicio es de carro, de moto o de ambos.").WithField("tipoVehiculo")
	ErrPrice    = shared.Validation("servicio_precio", "El precio va en pesos, sin decimales y no negativo.").WithField("precio")
	ErrDuration = shared.Validation("servicio_duracion", "La duración va en minutos, mayor que cero.").WithField("duracionMin")
)

// Service es un servicio que el taller ofrece. Precio nil = "el taller te confirma el valor" (HU-07).
type Service struct {
	ID          shared.ID
	TallerID    shared.ID
	Name        string
	Kind        string // carro | moto | ambos
	Price       *int64
	DurationMin *int
	Active      bool
	Version     int
}

func ValidKind(k string) bool { return k == kindCar || k == kindMoto || k == kindBoth }

// Validate revisa las reglas de un servicio.
func (s *Service) Validate() error {
	s.Name = strings.Join(strings.Fields(s.Name), " ")
	if s.Name == "" || len([]rune(s.Name)) > 120 {
		return ErrName
	}
	if !ValidKind(s.Kind) {
		return ErrKind
	}
	if s.Price != nil && *s.Price < 0 {
		return ErrPrice
	}
	if s.DurationMin != nil && *s.DurationMin <= 0 {
		return ErrDuration
	}
	return nil
}

const (
	kindCar  = "carro"
	kindMoto = "moto"
	kindBoth = "ambos"
)

// Precarga sin precio (HU-04): mantenimiento básico por tipo de vehículo. Propuesta editable;
// los de moto son provisionales (Q-02). Lo especializado sale de preloadBySpecialty.
var preloadCar = []string{
	"Cambio de aceite y filtro", "Revisión de frenos", "Cambio de pastillas de freno", "Revisión de suspensión",
	"Cambio de batería", "Sincronización", "Cambio de correa de distribución", "Cambio de bujías",
}

var preloadMoto = []string{
	"Cambio de aceite y filtro", "Revisión de frenos", "Cambio de pastillas de freno", "Ajuste y lubricación de cadena",
	"Cambio de kit de arrastre", "Sincronización", "Cambio de llantas", "Revisión de suspensión", "Cambio de batería",
}

// seed es un servicio semilla de una especialidad; kind dice a qué vehículos aplica.
type seed struct{ name, kind string }

// preloadBySpecialty agrega servicios según las especialidades del paso 2 (HU-03 → HU-04).
// Lista provisional (Q-02): el taller la edita en el paso 3. Las claves son las de tenancy.Specialties.
var preloadBySpecialty = map[string][]seed{
	"diagnostico_electronico": {{"Diagnóstico con escáner", kindBoth}, {"Borrado de códigos de falla", kindBoth}},
	"inyeccion_combustible":   {{"Limpieza de inyectores", kindBoth}, {"Cambio de filtro de combustible", kindBoth}, {"Revisión de bomba de combustible", kindCar}},
	"frenos_suspension":       {{"Cambio de líquido de frenos", kindBoth}, {"Rectificación de discos", kindCar}, {"Cambio de amortiguadores", kindBoth}},
	"caja_transmision":        {{"Cambio de aceite de caja", kindCar}, {"Cambio de kit de embrague", kindBoth}, {"Revisión de caja automática", kindCar}},
	"aire_acondicionado":      {{"Revisión de aire acondicionado", kindCar}, {"Carga de gas del aire acondicionado", kindCar}},
	"sistema_electrico":       {{"Revisión eléctrica", kindBoth}, {"Revisión de alternador y motor de arranque", kindBoth}, {"Cambio de bombillos", kindBoth}},
	"alto_voltaje":            {{"Diagnóstico de sistema híbrido o eléctrico", kindBoth}, {"Revisión de batería de alto voltaje", kindBoth}},
	"llantas_alineacion":      {{"Alineación y balanceo", kindCar}, {"Montaje de llantas", kindBoth}, {"Reparación de pinchazo", kindBoth}, {"Rotación de llantas", kindCar}},
	"latoneria_pintura":       {{"Latonería", kindBoth}, {"Pintura por pieza", kindBoth}, {"Pulida y brillada", kindBoth}, {"Arreglo de rayones", kindBoth}},
}

// preload acumula servicios sin repetir nombres; si un nombre llega de carro y de moto queda como "ambos".
type preload struct {
	kinds map[string]string
	order []string
}

func (p *preload) add(name, kind string) {
	prev, ok := p.kinds[name]
	if !ok {
		p.kinds[name] = kind
		p.order = append(p.order, name)
		return
	}
	if prev != kind {
		p.kinds[name] = kindBoth
	}
}

// kindFor dice cómo entra una semilla en un taller; "" = no aplica (p. ej. aire acondicionado en un taller de motos).
func kindFor(seedKind, attends string) string {
	switch {
	case attends == kindBoth:
		return seedKind
	case seedKind == kindBoth || seedKind == attends:
		return attends
	}
	return ""
}

// Preload devuelve los servicios iniciales según lo que atiende el taller y sus especialidades (HU-04).
// Con "ambos", el servicio que está en las dos listas queda como "ambos" (criterio 3).
func Preload(attends string, specialties []string) []Service {
	p := preload{kinds: map[string]string{}}
	if attends == kindCar || attends == kindBoth {
		for _, n := range preloadCar {
			p.add(n, kindCar)
		}
	}
	if attends == kindMoto || attends == kindBoth {
		for _, n := range preloadMoto {
			p.add(n, kindMoto)
		}
	}
	for _, sp := range specialties {
		for _, s := range preloadBySpecialty[sp] {
			if k := kindFor(s.kind, attends); k != "" {
				p.add(s.name, k)
			}
		}
	}
	out := make([]Service, 0, len(p.order))
	for _, n := range p.order {
		out = append(out, Service{Name: n, Kind: p.kinds[n], Active: true, Version: 1})
	}
	return out
}
