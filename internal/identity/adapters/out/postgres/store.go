// Package postgres implementa ports.Store con sqlc.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"sweveh/internal/identity/adapters/out/postgres/sqlcgen"
	"sweveh/internal/identity/domain"
	"sweveh/internal/identity/ports"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

type Store struct{}

func NewStore() Store { return Store{} }

func q(ctx context.Context) (*sqlcgen.Queries, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(conn), nil
}

func (Store) LatestChallengeTime(ctx context.Context, phone string) (*time.Time, error) {
	qs, err := q(ctx)
	if err != nil {
		return nil, err
	}
	t, err := qs.GetLatestOTPChallengeTime(ctx, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (Store) LatestOpenChallenge(ctx context.Context, phone string) (domain.Challenge, bool, error) {
	qs, err := q(ctx)
	if err != nil {
		return domain.Challenge{}, false, err
	}
	r, err := qs.GetLatestOpenOTPChallenge(ctx, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Challenge{}, false, nil
	}
	if err != nil {
		return domain.Challenge{}, false, err
	}
	return domain.Challenge{ID: r.ID, Phone: r.Celular, CodeHash: r.CodigoHash, ExpiresAt: r.ExpiraEn,
		UsedAt: r.UsadoEn, CreatedAt: r.CreatedAt}, true, nil
}

func (Store) InsertChallenge(ctx context.Context, c domain.Challenge) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.InsertOTPChallenge(ctx, sqlcgen.InsertOTPChallengeParams{
		ID: c.ID, Celular: c.Phone, CodigoHash: c.CodeHash, ExpiraEn: c.ExpiresAt, CreatedAt: c.CreatedAt,
	})
}

func (Store) CloseOpenChallenges(ctx context.Context, phone string, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.CloseOpenOTPChallenges(ctx, sqlcgen.CloseOpenOTPChallengesParams{Celular: phone, UsadoEn: &at})
}

func (Store) Lock(ctx context.Context, phone string) (domain.Lock, error) {
	qs, err := q(ctx)
	if err != nil {
		return domain.Lock{}, err
	}
	r, err := qs.GetOTPLock(ctx, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Lock{}, nil
	}
	if err != nil {
		return domain.Lock{}, err
	}
	return domain.Lock{Failures: int(r.Fallos), LockedUntil: r.BloqueadoHasta}, nil
}

func (Store) LockForUpdate(ctx context.Context, phone string) (domain.Lock, error) {
	qs, err := q(ctx)
	if err != nil {
		return domain.Lock{}, err
	}
	if err := qs.EnsureOTPLock(ctx, phone); err != nil {
		return domain.Lock{}, err
	}
	r, err := qs.GetOTPLockForUpdate(ctx, phone)
	if err != nil {
		return domain.Lock{}, err
	}
	return domain.Lock{Failures: int(r.Fallos), LockedUntil: r.BloqueadoHasta}, nil
}

func (Store) SaveLock(ctx context.Context, phone string, l domain.Lock, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	if err := qs.EnsureOTPLock(ctx, phone); err != nil {
		return err
	}
	return qs.UpdateOTPLock(ctx, sqlcgen.UpdateOTPLockParams{
		Celular: phone, Fallos: int32(l.Failures), BloqueadoHasta: l.LockedUntil, UpdatedAt: at,
	})
}

func (Store) UserByPhone(ctx context.Context, phone string) (ports.User, bool, error) {
	qs, err := q(ctx)
	if err != nil {
		return ports.User{}, false, err
	}
	r, err := qs.GetUserByPhone(ctx, phone)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.User{}, false, nil
	}
	if err != nil {
		return ports.User{}, false, err
	}
	return ports.User{ID: r.ID, Phone: r.Celular, Name: r.Nombre, Active: r.Activo}, true, nil
}

func (Store) UserByID(ctx context.Context, id shared.ID) (ports.User, error) {
	qs, err := q(ctx)
	if err != nil {
		return ports.User{}, err
	}
	r, err := qs.GetUserByID(ctx, id)
	if err != nil {
		return ports.User{}, db.NotFoundIfNoRows(err, domain.ErrSessionInvalid)
	}
	return ports.User{ID: r.ID, Phone: r.Celular, Name: r.Nombre, Active: r.Activo}, nil
}

func (Store) InsertUser(ctx context.Context, u ports.User, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	err = qs.InsertUser(ctx, sqlcgen.InsertUserParams{ID: u.ID, Celular: u.Phone, Nombre: u.Name, CreatedAt: at})
	if db.IsUniqueViolation(err, "usuario_celular_unico") {
		return domain.ErrAccountExists
	}
	return err
}

func (Store) InsertMembership(ctx context.Context, m ports.Membership, userID shared.ID, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.InsertMembership(ctx, sqlcgen.InsertMembershipParams{
		TallerID: m.TallerID, UsuarioID: userID, Rol: m.Role, CreatedAt: at,
	})
}

func (Store) ActiveMemberships(ctx context.Context, userID shared.ID) ([]ports.Membership, error) {
	qs, err := q(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := qs.ListActiveMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ports.Membership, 0, len(rows))
	for _, r := range rows {
		out = append(out, ports.Membership{TallerID: r.TallerID, Role: r.Rol})
	}
	return out, nil
}

func (Store) ActiveRole(ctx context.Context, tallerID, userID shared.ID) (string, bool, error) {
	qs, err := q(ctx)
	if err != nil {
		return "", false, err
	}
	rol, err := qs.GetActiveMembership(ctx, sqlcgen.GetActiveMembershipParams{TallerID: tallerID, UsuarioID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return rol, err == nil, err
}

func (Store) InsertConsent(ctx context.Context, c domain.Consent) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.InsertConsent(ctx, sqlcgen.InsertConsentParams{
		ID: c.ID, UsuarioID: c.UserID, Texto: c.Text, VersionPolitica: c.PolicyVersion, Canal: c.Channel,
		AceptadaEn: c.AcceptedAt,
	})
}

func (Store) InsertSession(ctx context.Context, s domain.Session, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.InsertSession(ctx, sqlcgen.InsertSessionParams{
		ID: s.ID, UsuarioID: s.UserID, TallerID: s.TallerID, CreatedAt: at, ExpiraAbsoluto: s.ExpiresAbs,
	})
}

func (Store) RevokeSession(ctx context.Context, id shared.ID, at time.Time, reason string) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.RevokeSession(ctx, sqlcgen.RevokeSessionParams{ID: id, RevocadaEn: &at, MotivoRevocacion: &reason})
}

func (Store) InsertRefresh(ctx context.Context, hash []byte, sessionID shared.ID, expires, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.InsertRefreshToken(ctx, sqlcgen.InsertRefreshTokenParams{Hash: hash, SesionID: sessionID, ExpiraEn: expires, CreatedAt: at})
}

func (Store) RefreshForUpdate(ctx context.Context, hash []byte) (ports.RefreshRecord, bool, error) {
	qs, err := q(ctx)
	if err != nil {
		return ports.RefreshRecord{}, false, err
	}
	r, err := qs.GetRefreshTokenForUpdate(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.RefreshRecord{}, false, nil
	}
	if err != nil {
		return ports.RefreshRecord{}, false, err
	}
	return ports.RefreshRecord{
		SessionID: r.SesionID, UserID: r.UsuarioID, TallerID: r.TallerID, ExpiresAt: r.ExpiraEn,
		UsedAt: r.UsadoEn, SessionAbs: r.ExpiraAbsoluto, RevokedAt: r.RevocadaEn,
	}, true, nil
}

func (Store) MarkRefreshUsed(ctx context.Context, hash []byte, at time.Time) error {
	qs, err := q(ctx)
	if err != nil {
		return err
	}
	return qs.MarkRefreshTokenUsed(ctx, sqlcgen.MarkRefreshTokenUsedParams{Hash: hash, UsadoEn: &at})
}
