// Package app contiene los casos de uso de identity: código por celular, registro del dueño,
// inicio de sesión, rotación del refresh con detección de reutilización y cierre de sesión.
package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/identity/domain"
	"sweveh/internal/identity/ports"
	"sweveh/internal/shared"
)

// Settings de Postgres que usa identity (ver db/migrations/00001_base.sql).
const (
	settingTaller  = "app.taller_id"
	settingUsuario = "app.usuario_id"
)

type Service struct {
	tx        ports.TxRunner
	store     ports.Store
	sender    ports.OTPSender
	tokens    ports.TokenIssuer
	workshops ports.Workshops
	audit     ports.AuditLog
	events    ports.EventPublisher
	clock     ports.Clock
	pepper    []byte // secreto del servidor para el HMAC del código
	log       *slog.Logger
	members   Members
}

type Deps struct {
	Tx        ports.TxRunner
	Store     ports.Store
	Sender    ports.OTPSender
	Tokens    ports.TokenIssuer
	Workshops ports.Workshops
	Audit     ports.AuditLog
	Events    ports.EventPublisher
	Clock     ports.Clock
	Pepper    []byte
	Log       *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{tx: d.Tx, store: d.Store, sender: d.Sender, tokens: d.Tokens, workshops: d.Workshops,
		audit: d.Audit, events: d.Events, clock: d.Clock, pepper: d.Pepper, log: d.Log}
}

// RequestCode genera y envía un código. La respuesta es la misma exista o no la cuenta.
func (s *Service) RequestCode(ctx context.Context, rawPhone string) (time.Time, error) {
	phone, err := shared.ParseColombianMobile(rawPhone)
	if err != nil {
		return time.Time{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	now := s.clock.Now()
	var code string
	ch := domain.Challenge{ID: shared.NewID(), Phone: phone, ExpiresAt: now.Add(domain.CodeTTL), CreatedAt: now}
	err = s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		lock, err := s.store.Lock(ctx, phone)
		if err != nil {
			return err
		}
		if lock.Locked(now) {
			return domain.ErrPhoneLocked
		}
		last, err := s.store.LatestChallengeTime(ctx, phone)
		if err != nil {
			return err
		}
		if last != nil && now.Sub(*last) < domain.ResendInterval {
			return domain.ErrResendTooSoon
		}
		if err := s.store.CloseOpenChallenges(ctx, phone, now); err != nil {
			return err
		}
		if code, err = randomCode(); err != nil {
			return err
		}
		ch.CodeHash = s.hashCode(ch.ID, phone, code)
		return s.store.InsertChallenge(ctx, ch)
	})
	if err != nil {
		return time.Time{}, err
	}
	// El envío va después del commit: nunca una llamada externa dentro de la transacción.
	if err := s.sender.Send(ctx, phone, code); err != nil {
		// La persona ve un mensaje claro; el detalle va al log (el error del canal no trae el código ni el celular).
		if s.log != nil {
			s.log.WarnContext(ctx, "no se pudo enviar el código de entrada", "error", err.Error())
		}
		return time.Time{}, domain.ErrCodeNotSent
	}
	return ch.ExpiresAt, nil
}

// VerifyInput es lo que llega al escribir el código.
type VerifyInput struct {
	Phone           string
	Code            string
	Name            string // solo para cuenta nueva
	ConsentAccepted bool   // solo para cuenta nueva (HU-02)
}

// Session es el resultado de entrar: tokens y datos para la pantalla.
type Session struct {
	AccessToken    string
	AccessExpires  time.Time
	RefreshToken   string
	RefreshExpires time.Time
	User           ports.User
	Workshop       ports.Workshop
	Roles          []string
	NewAccount     bool
}

// Verify comprueba el código y, según el caso, crea la cuenta o inicia sesión (HU-01).
func (s *Service) Verify(ctx context.Context, in VerifyInput) (Session, error) {
	phone, err := shared.ParseColombianMobile(in.Phone)
	if err != nil {
		return Session{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	if !domain.ValidCodeFormat(in.Code) {
		return Session{}, domain.ErrCodeFormat.WithField("codigo")
	}
	now := s.clock.Now()
	var (
		outcome  error // error de negocio que se devuelve DESPUÉS de guardar los fallos
		existing *ports.User
		name     string
	)
	err = s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		lock, err := s.store.LockForUpdate(ctx, phone)
		if err != nil {
			return err
		}
		if lock.Locked(now) {
			outcome = domain.ErrPhoneLocked
			return nil
		}
		ch, ok, err := s.store.LatestOpenChallenge(ctx, phone)
		if err != nil {
			return err
		}
		if !ok {
			outcome = domain.ErrCodeMissing
			return nil
		}
		if ch.Expired(now) {
			outcome = domain.ErrCodeExpired
			return nil
		}
		if !hmac.Equal(s.hashCode(ch.ID, phone, in.Code), ch.CodeHash) {
			next, remaining := lock.RegisterFailure(now)
			if err := s.store.SaveLock(ctx, phone, next, now); err != nil {
				return err
			}
			if remaining == 0 {
				outcome = domain.ErrPhoneLocked
				return s.store.CloseOpenChallenges(ctx, phone, now)
			}
			outcome = domain.ErrCodeWrong(remaining)
			return nil
		}
		u, found, err := s.store.UserByPhone(ctx, phone)
		if err != nil {
			return err
		}
		if found {
			existing = &u
		} else {
			// Cuenta nueva: el código es correcto, pero si faltan datos no se gasta (HU-02).
			if name, err = domain.ParsePersonName(in.Name); err != nil {
				outcome = domain.ErrRegistrationData
				return nil
			}
			if !in.ConsentAccepted {
				outcome = domain.ErrConsentRequired
				return nil
			}
		}
		if err := s.store.CloseOpenChallenges(ctx, phone, now); err != nil {
			return err
		}
		return s.store.SaveLock(ctx, phone, domain.Lock{}, now)
	})
	if err != nil {
		return Session{}, err
	}
	if outcome != nil {
		return Session{}, outcome
	}
	if existing != nil {
		return s.login(ctx, *existing, now)
	}
	return s.register(ctx, phone, name, now)
}

func (s *Service) register(ctx context.Context, phone, name string, now time.Time) (Session, error) {
	tallerID, userID := shared.NewID(), shared.NewID()
	actor := shared.Actor{Type: shared.ActorUser, ID: userID}
	var out Session
	settings := map[string]string{settingTaller: tallerID.String(), settingUsuario: userID.String()}
	err := s.tx.DoWithSettings(ctx, settings, func(ctx context.Context) error {
		ws, err := s.workshops.Create(ctx, tallerID, domain.DefaultWorkshopName(name), actor, now)
		if err != nil {
			return err
		}
		user := ports.User{ID: userID, Phone: phone, Name: name, Active: true}
		if err := s.store.InsertUser(ctx, user, now); err != nil {
			return err
		}
		if err := s.store.InsertMembership(ctx, ports.Membership{TallerID: tallerID, Role: string(domain.RoleOwner)}, userID, now); err != nil {
			return err
		}
		if err := s.store.InsertConsent(ctx, domain.Consent{
			ID: shared.NewID(), UserID: userID, Text: domain.ConsentText, PolicyVersion: domain.PolicyVersionDraft,
			Channel: domain.ConsentChannelWeb, AcceptedAt: now,
		}); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Entry{
			TallerID: tallerID, Actor: actor, Action: "usuario.registrar", Entity: "usuario", EntityID: userID,
			After:      map[string]any{"nombre": name, "rol": string(domain.RoleOwner), "politica": domain.PolicyVersionDraft},
			OccurredAt: now,
		}); err != nil {
			return err
		}
		if err := s.events.Publish(ctx, tallerID, shared.Event{
			Type: "identity.cuenta_creada", AggregateID: userID, OccurredAt: now,
		}); err != nil {
			return err
		}
		out, err = s.openSession(ctx, user, ws, []string{string(domain.RoleOwner)}, now)
		out.NewAccount = true
		return err
	})
	if err != nil {
		return Session{}, err
	}
	return out, nil
}

func (s *Service) login(ctx context.Context, user ports.User, now time.Time) (Session, error) {
	if !user.Active {
		return Session{}, domain.ErrNoMembership
	}
	var out Session
	err := s.tx.DoWithSettings(ctx, map[string]string{settingUsuario: user.ID.String()}, func(ctx context.Context) error {
		ms, err := s.store.ActiveMemberships(ctx, user.ID)
		if err != nil {
			return err
		}
		if len(ms) == 0 {
			return domain.ErrNoMembership
		}
		// Multi-sede está fuera del MVP: se entra al primer taller activo.
		m := ms[0]
		if err := s.tx.SetLocal(ctx, settingTaller, m.TallerID.String()); err != nil {
			return err
		}
		ws, err := s.workshops.Get(ctx, m.TallerID)
		if err != nil {
			return err
		}
		out, err = s.openSession(ctx, user, ws, []string{m.Role}, now)
		return err
	})
	return out, err
}

func (s *Service) openSession(ctx context.Context, user ports.User, ws ports.Workshop, roles []string, now time.Time) (Session, error) {
	sess := domain.Session{ID: shared.NewID(), UserID: user.ID, TallerID: ws.ID, ExpiresAbs: now.Add(domain.SessionMaxAge)}
	if err := s.store.InsertSession(ctx, sess, now); err != nil {
		return Session{}, err
	}
	raw, hash, err := newRefreshToken()
	if err != nil {
		return Session{}, err
	}
	refreshExp := minTime(now.Add(domain.RefreshTTL), sess.ExpiresAbs)
	if err := s.store.InsertRefresh(ctx, hash, sess.ID, refreshExp, now); err != nil {
		return Session{}, err
	}
	at, atExp, err := s.tokens.Issue(user.ID, ws.ID, sess.ID, roles)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: at, AccessExpires: atExp, RefreshToken: raw, RefreshExpires: refreshExp,
		User: user, Workshop: ws, Roles: roles}, nil
}

// Refresh rota el refresh token. Si llega uno ya usado, revoca la sesión completa (posible robo).
func (s *Service) Refresh(ctx context.Context, raw string) (Session, error) {
	if raw == "" {
		return Session{}, domain.ErrSessionInvalid
	}
	hash := hashRefresh(raw)
	now := s.clock.Now()
	var (
		out     Session
		outcome error
	)
	err := s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		rec, ok, err := s.store.RefreshForUpdate(ctx, hash)
		if err != nil {
			return err
		}
		if !ok {
			outcome = domain.ErrSessionInvalid
			return nil
		}
		if err := s.tx.SetLocal(ctx, settingTaller, rec.TallerID.String()); err != nil {
			return err
		}
		if err := s.tx.SetLocal(ctx, settingUsuario, rec.UserID.String()); err != nil {
			return err
		}
		if rec.UsedAt != nil {
			outcome = domain.ErrSessionInvalid
			s.log.WarnContext(ctx, "refresh reutilizado: se revoca la sesión", "sesion", rec.SessionID.String())
			if err := s.store.RevokeSession(ctx, rec.SessionID, now, "refresh_reutilizado"); err != nil {
				return err
			}
			return s.audit.Record(ctx, audit.Entry{
				TallerID: rec.TallerID, Actor: shared.Actor{Type: shared.ActorSystem}, Action: "sesion.revocar_por_reuso",
				Entity: "sesion", EntityID: rec.SessionID, OccurredAt: now,
			})
		}
		if rec.RevokedAt != nil || !now.Before(rec.SessionAbs) || !now.Before(rec.ExpiresAt) {
			outcome = domain.ErrSessionInvalid
			return nil
		}
		role, ok, err := s.store.ActiveRole(ctx, rec.TallerID, rec.UserID)
		if err != nil {
			return err
		}
		if !ok {
			outcome = domain.ErrSessionInvalid
			return s.store.RevokeSession(ctx, rec.SessionID, now, "membresia_inactiva")
		}
		if err := s.store.MarkRefreshUsed(ctx, hash, now); err != nil {
			return err
		}
		newRaw, newHash, err := newRefreshToken()
		if err != nil {
			return err
		}
		exp := minTime(now.Add(domain.RefreshTTL), rec.SessionAbs)
		if err := s.store.InsertRefresh(ctx, newHash, rec.SessionID, exp, now); err != nil {
			return err
		}
		user, err := s.store.UserByID(ctx, rec.UserID)
		if err != nil {
			return err
		}
		ws, err := s.workshops.Get(ctx, rec.TallerID)
		if err != nil {
			return err
		}
		at, atExp, err := s.tokens.Issue(rec.UserID, rec.TallerID, rec.SessionID, []string{role})
		if err != nil {
			return err
		}
		out = Session{AccessToken: at, AccessExpires: atExp, RefreshToken: newRaw, RefreshExpires: exp,
			User: user, Workshop: ws, Roles: []string{role}}
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	if outcome != nil {
		return Session{}, outcome
	}
	return out, nil
}

// Logout revoca la sesión del refresh. Es idempotente: sin token o ya revocada, no falla.
func (s *Service) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	hash := hashRefresh(raw)
	now := s.clock.Now()
	return s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		rec, ok, err := s.store.RefreshForUpdate(ctx, hash)
		if err != nil || !ok {
			return err
		}
		return s.store.RevokeSession(ctx, rec.SessionID, now, "cierre_de_sesion")
	})
}

// Me devuelve el usuario y su taller para la sesión actual.
func (s *Service) Me(ctx context.Context, userID, tallerID shared.ID) (ports.User, ports.Workshop, error) {
	var (
		u  ports.User
		ws ports.Workshop
	)
	err := s.tx.DoWithSettings(ctx, map[string]string{settingTaller: tallerID.String()}, func(ctx context.Context) error {
		var err error
		if u, err = s.store.UserByID(ctx, userID); err != nil {
			return err
		}
		ws, err = s.workshops.Get(ctx, tallerID)
		return err
	})
	return u, ws, err
}

func (s *Service) hashCode(challengeID shared.ID, phone, code string) []byte {
	m := hmac.New(sha256.New, s.pepper)
	m.Write([]byte(challengeID.String() + ":" + phone + ":" + code))
	return m.Sum(nil)
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// newRefreshToken devuelve el token opaco de 256 bits y su hash (solo el hash se guarda).
func newRefreshToken() (string, []byte, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b[:])
	return raw, hashRefresh(raw), nil
}

func hashRefresh(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
