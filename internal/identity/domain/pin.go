package domain

import (
	"fmt"
	"math"
	"time"

	"sweveh/internal/shared"
)

// PIN de entrada (HU-104, V-03, ADR-0037): 4 dígitos que elige la persona, sin verificar el celular.
// Con tan pocas combinaciones (10.000) lo que protege es el bloqueo por intentos y el hash lento (argon2id).
const PINLength = 4

var (
	ErrPINFormat   = shared.Validation("pin_formato", "El PIN tiene 4 números.").WithField("pin")
	ErrPINTrivial  = shared.Validation("pin_facil", "Ese PIN es muy fácil de adivinar. Escoge otros 4 números.").WithField("pin")
	ErrPINMismatch = shared.Validation("pin_actual_equivocado", "El PIN actual no coincide.").WithField("pinActual")
	// ErrWorkshopName: el nombre del taller se pide en el paso 1 del registro (V-04).
	ErrWorkshopName = shared.Validation("nombre_taller", "Escribe el nombre del taller (hasta 120 letras).").WithField("nombreTaller")
	// ErrPINAccountExists: el celular ya tiene cuenta; en el registro se le dice que entre con su PIN.
	ErrPINAccountExists = shared.Conflict("cuenta_existente", "Ese celular ya tiene cuenta. Entra con tu celular y tu PIN.").WithField("celular")
)

// trivialPINs son los PIN que se adivinan primero: repetidos y escaleras (lista corta, a propósito).
var trivialPINs = map[string]bool{
	"0000": true, "1111": true, "2222": true, "3333": true, "4444": true, "5555": true, "6666": true, "7777": true,
	"8888": true, "9999": true, "1234": true, "2345": true, "3456": true, "4567": true, "5678": true, "6789": true,
	"0123": true, "9876": true, "8765": true, "7654": true, "6543": true, "5432": true, "4321": true, "3210": true,
}

// ParsePIN valida que sean exactamente 4 dígitos y que no sea uno de los fáciles.
func ParsePIN(raw string) (string, error) {
	if len(raw) != PINLength {
		return "", ErrPINFormat
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return "", ErrPINFormat
		}
	}
	if trivialPINs[raw] {
		return "", ErrPINTrivial
	}
	return raw, nil
}

// ValidPINShape dice si tiene forma de PIN (4 dígitos), sin juzgar si es fácil: al entrar no se le dice a nadie
// que su PIN es "fácil", solo si coincide.
func ValidPINShape(raw string) bool {
	if len(raw) != PINLength {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseWorkshopName limpia y valida el nombre del taller.
func ParseWorkshopName(raw string) (string, error) {
	name, err := ParsePersonName(raw)
	if err != nil {
		return "", ErrWorkshopName
	}
	return name, nil
}

// LockPolicy es cuántos fallos se permiten y cuánto dura el bloqueo (PIN_MAX_INTENTOS, PIN_BLOQUEO).
type LockPolicy struct {
	MaxFailures int
	Duration    time.Duration
}

// DefaultLockPolicy es la de HU-01 y V-03: 5 fallos bloquean 15 minutos.
var DefaultLockPolicy = LockPolicy{MaxFailures: MaxFailures, Duration: LockDuration}

// RegisterFailureWith es RegisterFailure con la política dada.
func (l Lock) RegisterFailureWith(now time.Time, p LockPolicy) (Lock, int) {
	if l.LockedUntil != nil && !now.Before(*l.LockedUntil) {
		l = Lock{} // el bloqueo anterior ya venció: se empieza de cero
	}
	l.Failures++
	if l.Failures >= p.MaxFailures {
		until := now.Add(p.Duration)
		return Lock{Failures: 0, LockedUntil: &until}, 0
	}
	return l, p.MaxFailures - l.Failures
}

// ErrLoginFailed es el mismo error para un celular que no existe y para un PIN equivocado (no revela si la cuenta
// existe). El conteo de intentos va por celular, exista o no.
func ErrLoginFailed(remaining int) *shared.Error {
	msg := fmt.Sprintf("Celular o PIN incorrectos. Te quedan %d intentos; luego se bloquea 15 minutos.", remaining)
	if remaining == 1 {
		msg = "Celular o PIN incorrectos. Te queda 1 intento; luego se bloquea 15 minutos."
	}
	return shared.Validation("ingreso_fallido", msg)
}

// ErrLoginLocked dice cuántos minutos faltan para volver a intentar.
func ErrLoginLocked(now time.Time, l Lock) *shared.Error {
	mins := 1
	if l.LockedUntil != nil {
		mins = int(math.Ceil(l.LockedUntil.Sub(now).Minutes()))
		mins = max(mins, 1)
	}
	msg := fmt.Sprintf("Por seguridad bloqueamos este celular. Intenta de nuevo en %d minutos.", mins)
	if mins == 1 {
		msg = "Por seguridad bloqueamos este celular. Intenta de nuevo en 1 minuto."
	}
	return shared.RateLimited("celular_bloqueado", msg)
}
