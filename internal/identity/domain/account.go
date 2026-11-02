package domain

import (
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Datos de la autorización del dueño sobre sus propios datos (HU-02).
// La política aún no está publicada (HU-90) ni revisada por un abogado (AQ-10):
// la versión se marca como borrador para que nadie la confunda con la definitiva.
const (
	PolicyVersionDraft = "borrador-sin-validar-2026-11-01"
	ConsentText        = "Autorizo a SWEVEH a tratar mis datos personales según la política de tratamiento de datos."
	ConsentChannelWeb  = "web"
)

var (
	ErrNameRequired     = shared.Validation("nombre_requerido", "Escribe tu nombre.").WithField("nombre")
	ErrNameTooLong      = shared.Validation("nombre_largo", "El nombre puede tener hasta 120 letras.").WithField("nombre")
	ErrConsentRequired  = shared.Rule("autorizacion_requerida", "Para continuar debes aceptar la política de tratamiento de datos.").WithField("autorizacion")
	ErrRegistrationData = shared.Rule("registro_incompleto", "Es tu primera vez: escribe tu nombre y acepta la política de datos para crear la cuenta.")
	ErrNoMembership     = shared.Forbidden("sin_taller", "Tu usuario no está activo en ningún taller.")
	ErrSessionInvalid   = shared.Unauthorized("sesion_invalida", "Tu sesión terminó. Vuelve a entrar con tu celular.")
	// Dos registros simultáneos con el mismo celular: el segundo recibe este error.
	ErrAccountExists = shared.Conflict("cuenta_existente", "Ya hay una cuenta con este celular. Entra con tu código.")
)

// ParsePersonName limpia y valida un nombre de persona.
func ParsePersonName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return "", ErrNameRequired
	}
	if len([]rune(name)) > 120 {
		return "", ErrNameTooLong
	}
	return name, nil
}

// DefaultWorkshopName es el nombre provisional del taller hasta que el dueño lo cambie (HU-06:
// los datos se piden cuando hagan falta).
func DefaultWorkshopName(ownerName string) string {
	first := strings.Fields(ownerName)
	if len(first) == 0 {
		return "Mi taller"
	}
	return "Taller de " + first[0]
}

// Consent es la prueba de la autorización: texto, versión, canal, fecha y hora.
type Consent struct {
	ID            shared.ID
	UserID        shared.ID
	Text          string
	PolicyVersion string
	Channel       string
	AcceptedAt    time.Time
}

// Session agrupa una familia de refresh tokens.
type Session struct {
	ID         shared.ID
	UserID     shared.ID
	TallerID   shared.ID
	ExpiresAbs time.Time
	RevokedAt  *time.Time
}

// Active indica si la sesión sigue viva.
func (s Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAbs)
}

// Duraciones del refresh (docs/06 §6.3, Propuesta): deslizante de 14 días con máximo de 30.
const (
	RefreshTTL    = 14 * 24 * time.Hour
	SessionMaxAge = 30 * 24 * time.Hour
)
