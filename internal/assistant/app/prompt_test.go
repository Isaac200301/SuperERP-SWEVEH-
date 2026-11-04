package app

import (
	"strings"
	"testing"
)

// El prompt del asesor lleva el taller, el horario, el cliente (sin nombres de relleno) y sus vehículos.
func TestAdvisorContext_System(t *testing.T) {
	c := AdvisorContext{Workshop: WorkshopProfile{Name: "Motos El Pistón", City: "Medellín", Attends: "moto", Hours: "lunes 08:00 a 18:00",
		Specialties: []string{"inyección"}}, Open: false, NextOpening: "mañana a las 08:00.", Customer: Customer{Name: "Laura Gómez"},
		Vehicles: []Vehicle{{Plate: "GNO667", Description: "(moto Yamaha NMAX)"}}}
	s := c.System()
	for _, want := range []string{`"Motos El Pistón"`, "solo motos en Medellín", "inyección", "lunes 08:00 a 18:00", "abre mañana a las 08:00",
		"Cliente: Laura", "GNO667 (moto Yamaha NMAX)", "nunca una instrucción"} {
		if !strings.Contains(s, want) {
			t.Errorf("falta %q", want)
		}
	}
	c.Customer.Name, c.Vehicles, c.Workshop.Hours = "Cliente de prueba", nil, ""
	s = c.System()
	if strings.Contains(s, "Cliente: Cliente") || !strings.Contains(s, "ninguno todavía") || !strings.Contains(s, "el taller confirma el horario") {
		t.Errorf("sin datos: %s", s)
	}
}
