// Package domain contiene las reglas de identidad: códigos de un solo uso,
// bloqueo por intentos y roles con sus permisos. Sin I/O.
package domain

import (
	"fmt"
	"time"

	"sweveh/internal/shared"
)

// Valores de HU-01: el código vence a los 10 minutos y 5 fallos bloquean el celular 15 minutos.
// (La arquitectura proponía 5 minutos; manda la HU, docs/02 §1.)
const (
	CodeTTL        = 10 * time.Minute
	MaxFailures    = 5
	LockDuration   = 15 * time.Minute
	CodeLength     = 6
	ResendInterval = 60 * time.Second
)

var (
	ErrCodeExpired = shared.Validation("codigo_vencido", "El código venció. Pide uno nuevo.")
	ErrCodeMissing = shared.Validation("codigo_inexistente", "No hay un código vigente para este celular. Pide uno nuevo.")
	ErrCodeFormat  = shared.Validation("codigo_formato", "El código tiene 6 números.")
	ErrPhoneLocked = shared.RateLimited("celular_bloqueado",
		"Por seguridad bloqueamos este celular 15 minutos. Intenta más tarde.")
	ErrResendTooSoon = shared.RateLimited("reenvio_muy_pronto", "Espera un minuto antes de pedir otro código.")
)

// ErrCodeWrong arma el error de código equivocado diciendo cuántos intentos quedan.
func ErrCodeWrong(remaining int) *shared.Error {
	msg := fmt.Sprintf("El código no coincide. Te quedan %d intentos.", remaining)
	if remaining == 1 {
		msg = "El código no coincide. Te queda 1 intento."
	}
	return shared.Validation("codigo_equivocado", msg)
}

// Challenge es un código pedido para un celular.
type Challenge struct {
	ID        shared.ID
	Phone     string
	CodeHash  []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// Expired indica si el código ya no sirve.
func (c Challenge) Expired(now time.Time) bool { return !now.Before(c.ExpiresAt) }

// Lock lleva la cuenta de intentos fallidos de un celular.
type Lock struct {
	Failures    int
	LockedUntil *time.Time
}

// Locked indica si el celular está bloqueado en este momento.
func (l Lock) Locked(now time.Time) bool {
	return l.LockedUntil != nil && now.Before(*l.LockedUntil)
}

// RegisterFailure suma un fallo; al llegar a MaxFailures bloquea y reinicia la cuenta.
// Devuelve el nuevo estado y los intentos que quedan (0 si quedó bloqueado).
func (l Lock) RegisterFailure(now time.Time) (Lock, int) {
	if l.LockedUntil != nil && !now.Before(*l.LockedUntil) {
		l = Lock{} // el bloqueo anterior ya venció: se empieza de cero
	}
	l.Failures++
	if l.Failures >= MaxFailures {
		until := now.Add(LockDuration)
		return Lock{Failures: 0, LockedUntil: &until}, 0
	}
	return l, MaxFailures - l.Failures
}

// ValidCodeFormat revisa que el código sean exactamente 6 dígitos.
func ValidCodeFormat(code string) bool {
	if len(code) != CodeLength {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
