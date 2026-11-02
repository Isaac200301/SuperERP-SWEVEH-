// Package ports declara lo que los casos de uso de identity necesitan del exterior.
package ports

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/identity/domain"
	"sweveh/internal/shared"
)

// User es una persona que entra con su celular.
type User struct {
	ID     shared.ID
	Phone  string
	Name   string
	Active bool
}

// Membership es el rol del usuario en un taller.
type Membership struct {
	TallerID shared.ID
	Role     string
}

// RefreshRecord es un refresh token con su sesión.
type RefreshRecord struct {
	SessionID  shared.ID
	UserID     shared.ID
	TallerID   shared.ID
	ExpiresAt  time.Time
	UsedAt     *time.Time
	SessionAbs time.Time
	RevokedAt  *time.Time
}

// Store persiste OTP, usuarios, membresías y sesiones. Usa la transacción del context.
type Store interface {
	LatestChallengeTime(ctx context.Context, phone string) (*time.Time, error)
	LatestOpenChallenge(ctx context.Context, phone string) (domain.Challenge, bool, error)
	InsertChallenge(ctx context.Context, c domain.Challenge) error
	CloseOpenChallenges(ctx context.Context, phone string, at time.Time) error

	Lock(ctx context.Context, phone string) (domain.Lock, error)
	// LockForUpdate crea la fila si no existe y la bloquea hasta el commit.
	LockForUpdate(ctx context.Context, phone string) (domain.Lock, error)
	SaveLock(ctx context.Context, phone string, l domain.Lock, at time.Time) error

	UserByPhone(ctx context.Context, phone string) (User, bool, error)
	UserByID(ctx context.Context, id shared.ID) (User, error)
	InsertUser(ctx context.Context, u User, at time.Time) error
	InsertMembership(ctx context.Context, m Membership, userID shared.ID, at time.Time) error
	// ActiveMemberships requiere app.usuario_id fijado.
	ActiveMemberships(ctx context.Context, userID shared.ID) ([]Membership, error)
	ActiveRole(ctx context.Context, tallerID, userID shared.ID) (string, bool, error)
	InsertConsent(ctx context.Context, c domain.Consent) error

	InsertSession(ctx context.Context, s domain.Session, at time.Time) error
	RevokeSession(ctx context.Context, id shared.ID, at time.Time, reason string) error
	InsertRefresh(ctx context.Context, hash []byte, sessionID shared.ID, expires, at time.Time) error
	RefreshForUpdate(ctx context.Context, hash []byte) (RefreshRecord, bool, error)
	MarkRefreshUsed(ctx context.Context, hash []byte, at time.Time) error
}

// TxRunner abre transacciones de autenticación (antes de conocer el taller).
type TxRunner interface {
	DoWithSettings(ctx context.Context, settings map[string]string, fn func(ctx context.Context) error) error
	SetLocal(ctx context.Context, name, value string) error
}

// OTPSender entrega el código. Canal por decidir (Q-13/AQ-03): SMS o plantilla de WhatsApp.
type OTPSender interface {
	Send(ctx context.Context, phone, code string) error
}

// TokenIssuer firma el token de acceso de 10 minutos.
type TokenIssuer interface {
	Issue(userID, tallerID, sessionID shared.ID, roles []string) (value string, expires time.Time, err error)
}

// Workshop es lo que identity ve del taller.
type Workshop struct {
	ID   shared.ID
	Name string
}

// Workshops es la fachada de tenancy vista desde identity.
type Workshops interface {
	Create(ctx context.Context, id shared.ID, name string, actor shared.Actor, now time.Time) (Workshop, error)
	Get(ctx context.Context, id shared.ID) (Workshop, error)
}

type AuditLog interface {
	Record(ctx context.Context, e audit.Entry) error
}

type EventPublisher interface {
	Publish(ctx context.Context, tallerID shared.ID, e shared.Event) error
}

type Clock interface{ Now() time.Time }
