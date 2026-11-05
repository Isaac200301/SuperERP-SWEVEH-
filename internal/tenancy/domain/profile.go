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
	ErrAttends  = shared.Validation("atiende_requerido", "Elige qué vehículos atiendes: carros, motos o ambos.").WithField("atiende")
	ErrSize     = shared.Validation("tamano_invalido", "Elige cuántos trabajan: solo yo, 2 a 5 o 6 o más.").WithField("tamano")
	ErrCity     = shared.Validation("ciudad_invalida", "La ciudad puede tener hasta 80 letras.").WithField("ciudad")
	ErrAddress  = shared.Validation("direccion_invalida", "La dirección puede tener hasta 160 caracteres.").WithField("direccion")
	ErrContract = shared.Rule("contrato_requerido", "Para continuar acepta el contrato de transmisión de datos con SWEVEH.").WithField("contratoAceptado")
	ErrSchedule = shared.Validation("horario_invalido", "Usa horas como 08:00 y la hora de cierre después de la de apertura.").WithField("horario")
)

var Days = []string{"lun", "mar", "mie", "jue", "vie", "sab", "dom"}

// Specialties son las opciones del paso 2. "sistema_electrico" (luces, arranque, carga) y "alto_voltaje"
// (híbridos y eléctricos, exige certificación) se separan a propósito: son trabajos muy distintos.
var Specialties = []string{"mecanica_general", "diagnostico_electronico", "inyeccion_combustible", "frenos_suspension", "caja_transmision",
	"aire_acondicionado", "sistema_electrico", "alto_voltaje", "llantas_alineacion", "latoneria_pintura", "otro"}

// legacySpecialties traduce las claves de la primera versión del registro (migración 00010).
var legacySpecialties = map[string]string{"electrico": "sistema_electrico", "diagnostico_escaner": "diagnostico_electronico"}

// Profile son los datos del paso 2 (HU-03).
type Profile struct {
	Attends     string
	Specialties []string
	City        string
	Size        string
	Address     string // opcional: sale en el recordatorio de la cita (HU-44)
}

func (p *Profile) Validate() error {
	if p.Attends != "carro" && p.Attends != "moto" && p.Attends != "ambos" {
		return ErrAttends
	}
	if p.Size != "solo" && p.Size != "2a5" && p.Size != "6mas" {
		return ErrSize
	}
	p.City = normalizeCity(p.City)
	if len([]rune(p.City)) > 80 {
		return ErrCity
	}
	p.Address = strings.Join(strings.Fields(p.Address), " ")
	if len([]rune(p.Address)) > 160 {
		return ErrAddress
	}
	clean := []string{}
	for _, s := range p.Specialties {
		if n, ok := legacySpecialties[s]; ok {
			s = n
		}
		if slices.Contains(Specialties, s) && !slices.Contains(clean, s) {
			clean = append(clean, s)
		}
	}
	p.Specialties = clean
	return nil
}

// normalizeCity quita espacios de más y pone mayúscula inicial en cada palabra ("medellin" → "Medellin"),
// salvo conectores ("Santa Rosa de Osos"). No corrige tildes: eso pide un selector de municipios (Sin dato).
func normalizeCity(c string) string {
	words := strings.Fields(strings.ToLower(c))
	for i, w := range words {
		if i > 0 && slices.Contains([]string{"de", "del", "la", "las", "los", "el", "y"}, w) {
			continue
		}
		r := []rune(w)
		words[i] = strings.ToUpper(string(r[0])) + string(r[1:])
	}
	return strings.Join(words, " ")
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
