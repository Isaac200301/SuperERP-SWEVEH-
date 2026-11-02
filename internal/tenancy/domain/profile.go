package domain

import (
	"regexp"
	"slices"
	"strings"

	"sweveh/internal/shared"
)

// Versión del contrato de transmisión de datos taller → SWEVEH (HU-89). Borrador hasta AQ-10.
const ContractVersionDraft = "contrato-transmision-borrador-2026-11-01"

var (
	ErrAttends   = shared.Validation("atiende_requerido", "Elige qué vehículos atiendes: carros, motos o ambos.").WithField("atiende")
	ErrSize      = shared.Validation("tamano_invalido", "Elige cuántos trabajan: solo yo, 2 a 5 o 6 o más.").WithField("tamano")
	ErrCity      = shared.Validation("ciudad_invalida", "La ciudad puede tener hasta 80 letras.").WithField("ciudad")
	ErrContract  = shared.Rule("contrato_requerido", "Para continuar acepta el contrato de transmisión de datos con SWEVEH.").WithField("contratoAceptado")
	ErrSchedule  = shared.Validation("horario_invalido", "Usa horas como 08:00 y la hora de cierre después de la de apertura.").WithField("horario")
)

var Days = []string{"lun", "mar", "mie", "jue", "vie", "sab", "dom"}

// Specialties son las opciones del paso 2 (mockup Registro). Otras se guardan como "otro".
var Specialties = []string{"mecanica_general", "frenos_suspension", "electrico", "diagnostico_escaner", "llantas_alineacion", "latoneria_pintura", "otro"}

// Profile son los datos del paso 2 (HU-03).
type Profile struct {
	Attends     string
	Specialties []string
	City        string
	Size        string
}

func (p *Profile) Validate() error {
	if p.Attends != "carro" && p.Attends != "moto" && p.Attends != "ambos" {
		return ErrAttends
	}
	if p.Size != "solo" && p.Size != "2a5" && p.Size != "6mas" {
		return ErrSize
	}
	p.City = strings.Join(strings.Fields(p.City), " ")
	if len([]rune(p.City)) > 80 {
		return ErrCity
	}
	clean := []string{}
	for _, s := range p.Specialties {
		if slices.Contains(Specialties, s) && !slices.Contains(clean, s) {
			clean = append(clean, s)
		}
	}
	p.Specialties = clean
	return nil
}

// Slot es la franja de un día. Nil = cerrado.
type Slot struct {
	Opens  string `json:"abre"`
	Closes string `json:"cierra"`
}

// Schedule es el horario semanal (HU-08).
type Schedule map[string]*Slot

var reHour = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func (s Schedule) Validate() error {
	for k, v := range s {
		if !slices.Contains(Days, k) {
			return ErrSchedule
		}
		if v == nil {
			continue
		}
		if !reHour.MatchString(v.Opens) || !reHour.MatchString(v.Closes) || v.Opens >= v.Closes {
			return ErrSchedule
		}
	}
	return nil
}
