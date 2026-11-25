package app

import (
	"context"
	"errors"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/identity/domain"
	"sweveh/internal/identity/ports"
	"sweveh/internal/shared"
)

// Entrada con celular y PIN (HU-104, V-03, ADR-0037). El celular no se verifica (riesgo aceptado para la entrega,
// Q-40). Los fallos se cuentan por celular en el mismo candado de HU-01 (fila bloqueada con FOR UPDATE: dos intentos
// a la vez no se pierden), exista o no la cuenta, y el error es el mismo en los dos casos.

// OwnerDefaultName es el nombre del dueño mientras no lo escriba: el registro de V-04 solo pide el nombre del taller.
// Se cambia en Equipo (decisión provisional, docs/07).
const OwnerDefaultName = "Dueño"

// RegisterPINInput es el paso 1 del registro (V-04, Figma "02 Registro · paso 1").
type RegisterPINInput struct {
	Phone        string
	WorkshopName string
	PersonName   string // opcional: sin nombre queda OwnerDefaultName
	PIN          string
	Consent      bool   // HU-02: la casilla sin marcar no deja seguir
	PolicyVer    string // la versión que vio la persona; debe ser la vigente
}

// RegisterWithPIN crea la cuenta del dueño, su taller y la prueba de la autorización, y abre la sesión.
func (s *Service) RegisterWithPIN(ctx context.Context, in RegisterPINInput) (Session, error) {
	phone, err := shared.ParseColombianMobile(in.Phone)
	if err != nil {
		return Session{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	ws, err := domain.ParseWorkshopName(in.WorkshopName)
	if err != nil {
		return Session{}, err
	}
	name := OwnerDefaultName
	if in.PersonName != "" {
		if name, err = domain.ParsePersonName(in.PersonName); err != nil {
			return Session{}, err
		}
	}
	pin, err := domain.ParsePIN(in.PIN)
	if err != nil {
		return Session{}, err
	}
	if !in.Consent || (in.PolicyVer != "" && in.PolicyVer != domain.PolicyVersionDraft) {
		return Session{}, domain.ErrConsentRequired
	}
	hash, err := s.hasher.Hash(pin) // lento a propósito: fuera de la transacción
	if err != nil {
		return Session{}, err
	}
	out, err := s.register(ctx, newAccount{phone: phone, name: name, workshop: ws, pinHash: hash}, s.clock.Now())
	if errors.Is(err, domain.ErrAccountExists) {
		return Session{}, domain.ErrPINAccountExists
	}
	return out, err
}

// LoginWithPIN entra con celular y PIN. 5 fallos (PIN_MAX_INTENTOS) bloquean el celular 15 minutos (PIN_BLOQUEO).
func (s *Service) LoginWithPIN(ctx context.Context, rawPhone, pin string) (Session, error) {
	phone, err := shared.ParseColombianMobile(rawPhone)
	if err != nil {
		return Session{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	if !domain.ValidPINShape(pin) {
		return Session{}, domain.ErrPINFormat
	}
	now := s.clock.Now()
	// 1. Transacción corta: con la fila del celular bloqueada se RESERVA el intento (cuenta como fallo hasta que se
	// pruebe lo contrario). Así nunca hay más de 5 intentos en vuelo y ninguna conexión espera al argon2 (R-07).
	var (
		outcome   error
		user      ports.User
		found     bool
		next      domain.Lock
		remaining int
	)
	err = s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		lock, err := s.store.LockForUpdate(ctx, phone)
		if err != nil {
			return err
		}
		if lock.Locked(now) {
			outcome = domain.ErrLoginLocked(now, lock)
			return nil
		}
		if user, found, err = s.store.UserByPhone(ctx, phone); err != nil {
			return err
		}
		next, remaining = lock.RegisterFailureWith(now, s.lockPolicy)
		return s.store.SaveLock(ctx, phone, next, now)
	})
	if err != nil {
		return Session{}, err
	}
	if outcome != nil {
		return Session{}, outcome
	}
	// 2. El hash lento, fuera de la transacción. Verify corre SIEMPRE, también sin cuenta (contra un relleno): no se
	// cambie el orden a `!found || ...`, que haría más rápida la respuesta de un celular sin cuenta.
	if ok := s.hasher.Verify(pin, user.PINHash); !ok || !found {
		if remaining == 0 {
			outcome = domain.ErrLoginLocked(now, next)
		} else {
			outcome = domain.ErrLoginFailed(remaining)
		}
		if found {
			action := "cuenta.ingreso_fallido"
			if remaining == 0 {
				action = "cuenta.bloqueo"
			}
			if err := s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
				return s.auditAccount(ctx, user, action, map[string]any{"quedan": remaining}, now)
			}); err != nil {
				return Session{}, err
			}
		}
		return Session{}, outcome
	}
	// 3. Acertó: se devuelve el intento reservado (contador en cero) y se abre la sesión.
	if err := s.tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		if _, err := s.store.LockForUpdate(ctx, phone); err != nil {
			return err
		}
		return s.store.SaveLock(ctx, phone, domain.Lock{}, now)
	}); err != nil {
		return Session{}, err
	}
	return s.loginVia(ctx, user, now, "pin")
}

// ChangePIN cambia el PIN propio. Pide el actual, salvo si el dueño puso uno temporal (pin_cambiar). Un PIN actual
// equivocado cuenta como intento fallido del celular.
func (s *Service) ChangePIN(ctx context.Context, userID, tallerID shared.ID, current, next string) error {
	pin, err := domain.ParsePIN(next)
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(pin)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	var outcome error
	err = s.tx.DoWithSettings(ctx, map[string]string{settingTaller: tallerID.String(), settingUsuario: userID.String()}, func(ctx context.Context) error {
		u, err := s.store.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		lock, err := s.store.LockForUpdate(ctx, u.Phone)
		if err != nil {
			return err
		}
		if lock.Locked(now) {
			outcome = domain.ErrLoginLocked(now, lock)
			return nil
		}
		if !u.MustChangePIN && !s.hasher.Verify(current, u.PINHash) {
			nl, remaining := lock.RegisterFailureWith(now, s.lockPolicy)
			outcome = domain.ErrPINMismatch
			if remaining == 0 {
				outcome = domain.ErrLoginLocked(now, nl)
			}
			return s.store.SaveLock(ctx, u.Phone, nl, now)
		}
		if err := s.store.SetPIN(ctx, userID, hash, false, now); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: shared.Actor{Type: shared.ActorUser, ID: userID},
			Action: "cuenta.pin_cambio", Entity: "usuario", EntityID: userID, OccurredAt: now})
	})
	if err != nil {
		return err
	}
	return outcome
}

// ResetPIN pone un PIN temporal a alguien del equipo (solo quien tiene user:manage). Se le dice de palabra; al entrar
// debe cambiarlo. Desbloquea el celular y cierra sus sesiones abiertas. Nunca a uno mismo: para eso está ChangePIN.
func (s *Service) ResetPIN(ctx context.Context, tallerID shared.ID, actor shared.Actor, userID shared.ID, temp string) error {
	if userID == actor.ID {
		return ErrSelf
	}
	pin, err := domain.ParsePIN(temp)
	if err != nil {
		return err
	}
	hash, err := s.hasher.Hash(pin)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	return s.tx.DoWithSettings(ctx, map[string]string{settingTaller: tallerID.String()}, func(ctx context.Context) error {
		if _, err := s.members.Get(ctx, tallerID, userID); err != nil {
			return err
		}
		if err := s.onlyThisWorkshop(ctx, tallerID, userID); err != nil {
			return err
		}
		u, err := s.store.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		if err := s.store.SetPIN(ctx, userID, hash, true, now); err != nil {
			return err
		}
		if _, err := s.store.LockForUpdate(ctx, u.Phone); err != nil {
			return err
		}
		if err := s.store.SaveLock(ctx, u.Phone, domain.Lock{}, now); err != nil {
			return err
		}
		if err := s.members.RevokeSessions(ctx, tallerID, userID, now, "pin_restablecido"); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "cuenta.pin_reset", Entity: "usuario",
			EntityID: userID, OccurredAt: now})
	})
}

// ErrPINOtherWorkshop: el PIN es de la persona y sirve en todos sus talleres; el dueño de uno no se lo cambia a quien
// también está en otro (podría entrar como esa persona allá). Lo cambia ella misma o un administrador (make pin-reset).
var ErrPINOtherWorkshop = shared.Rule("pin_de_otro_taller",
	"Esa persona también trabaja en otro taller: su PIN lo cambia ella misma desde su cuenta o un administrador de SWEVEH.")

// onlyThisWorkshop falla si el usuario tiene membresía activa en otro taller (R-05, R-06).
func (s *Service) onlyThisWorkshop(ctx context.Context, tallerID, userID shared.ID) error {
	if err := s.tx.SetLocal(ctx, settingUsuario, userID.String()); err != nil {
		return err
	}
	ms, err := s.store.ActiveMemberships(ctx, userID)
	if err != nil {
		return err
	}
	for _, m := range ms {
		if m.TallerID != tallerID {
			return ErrPINOtherWorkshop
		}
	}
	return nil
}

// auditAccount deja en la bitácora de su taller lo que le pasó a una cuenta al entrar (sin el PIN). La bitácora es
// por taller (RLS): se usa el primer taller activo del usuario; sin taller no hay dónde anotarlo.
func (s *Service) auditAccount(ctx context.Context, u ports.User, action string, after map[string]any, now time.Time) error {
	if err := s.tx.SetLocal(ctx, settingUsuario, u.ID.String()); err != nil {
		return err
	}
	ms, err := s.store.ActiveMemberships(ctx, u.ID)
	if err != nil || len(ms) == 0 {
		return err
	}
	if err := s.tx.SetLocal(ctx, settingTaller, ms[0].TallerID.String()); err != nil {
		return err
	}
	return s.audit.Record(ctx, audit.Entry{TallerID: ms[0].TallerID, Actor: shared.Actor{Type: shared.ActorUser, ID: u.ID},
		Action: action, Entity: "usuario", EntityID: u.ID, After: after, OccurredAt: now})
}
