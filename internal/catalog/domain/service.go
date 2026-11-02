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

func ValidKind(k string) bool { return k == "carro" || k == "moto" || k == "ambos" }

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

// Precarga sin precio (HU-04). Propuesta editable: los de moto son provisionales (Q-02).
var preloadCar = []string{
	"Cambio de aceite y filtro", "Revisión de frenos", "Cambio de pastillas de freno", "Alineación y balanceo",
	"Revisión de suspensión", "Diagnóstico con escáner", "Cambio de batería", "Sincronización",
	"Revisión eléctrica", "Cambio de correa de distribución", "Revisión de aire acondicionado", "Cambio de bujías",
}

var preloadMoto = []string{
	"Cambio de aceite y filtro", "Revisión de frenos", "Cambio de pastillas de freno", "Ajuste y lubricación de cadena",
	"Cambio de kit de arrastre", "Sincronización", "Revisión eléctrica", "Cambio de llantas",
	"Revisión de suspensión", "Cambio de batería",
}

// Preload devuelve los servicios iniciales según lo que atiende el taller. Con "ambos",
// el servicio que está en las dos listas queda como "ambos" (HU-04, criterio 3).
func Preload(attends string) []Service {
	kinds := map[string]string{}
	order := []string{}
	add := func(names []string, kind string) {
		for _, n := range names {
			if prev, ok := kinds[n]; ok {
				if prev != kind {
					kinds[n] = "ambos"
				}
				continue
			}
			kinds[n] = kind
			order = append(order, n)
		}
	}
	if attends == "carro" || attends == "ambos" {
		add(preloadCar, "carro")
	}
	if attends == "moto" || attends == "ambos" {
		add(preloadMoto, "moto")
	}
	out := make([]Service, 0, len(order))
	for _, n := range order {
		out = append(out, Service{Name: n, Kind: kinds[n], Active: true, Version: 1})
	}
	return out
}
