package app

import (
	"context"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/identity/domain"
	"sweveh/internal/identity/ports"
	"sweveh/internal/shared"
)

// Member es un usuario del taller con su rol.
type Member struct {
	UserID    shared.ID
	Name      string
	Phone     string
	Role      string
	Active    bool
	CreatedAt time.Time
}

// Members persiste la relación usuario-taller (HU-80, HU-84).
type Members interface {
	List(ctx context.Context, tallerID shared.ID) ([]Member, error)
	Get(ctx context.Context, tallerID, userID shared.ID) (Member, error)
	Upsert(ctx context.Context, tallerID, userID shared.ID, role string, at time.Time) error
	SetActive(ctx context.Context, tallerID, userID shared.ID, active bool) error
	RevokeSessions(ctx context.Context, tallerID, userID shared.ID, at time.Time, reason string) error
	Workshops(ctx context.Context, userID shared.ID) ([]ports.Workshop, error)
}

var (
	ErrRole      = shared.Validation("rol_invalido", "El rol debe ser mecánico o recepción.").WithField("rol")
	ErrMemberNF  = shared.NotFound("usuario_no_encontrado", "Ese usuario no está en tu taller.")
	ErrSelf      = shared.Rule("no_a_ti_mismo", "No puedes desactivarte ni cambiar tu propio rol.")
	ErrNotMember = shared.Forbidden("sin_taller", "No perteneces a ese taller.")
)

// SetMembers conecta el repositorio de miembros.
func (s *Service) SetMembers(m Members) { s.members = m }

func (s *Service) ListMembers(ctx context.Context, tallerID shared.ID) ([]Member, error) {
	var out []Member
	err := s.tx.DoWithSettings(ctx, map[string]string{settingTaller: tallerID.String()}, func(ctx context.Context) error {
		var err error
		out, err = s.members.List(ctx, tallerID)
		return err
	})
	return out, err
}

// AddMember agrega un mecánico o recepción por celular (HU-80, HU-82). Si ya es usuario de otro taller,
// queda en los dos y escoge en cuál trabaja al entrar.
func (s *Service) AddMember(ctx context.Context, tallerID shared.ID, actor shared.Actor, rawPhone, rawName, role string) (Member, error) {
	if role != string(domain.RoleMechanic) && role != string(domain.RoleReception) {
		return Member{}, ErrRole
	}
	phone, err := shared.ParseColombianMobile(rawPhone)
	if err != nil {
		return Member{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	name, err := domain.ParsePersonName(rawName)
	if err != nil {
		return Member{}, err
	}
	now := s.clock.Now()
	var out Member
	err = s.tx.DoWithSettings(ctx, map[string]string{settingTaller: tallerID.String()}, func(ctx context.Context) error {
		u, found, err := s.store.UserByPhone(ctx, phone)
		if err != nil {
			return err
		}
		if !found {
			u = ports.User{ID: shared.NewID(), Phone: phone, Name: name, Active: true}
			if err := s.store.InsertUser(ctx, u, now); err != nil {
				return err
			}
		}
		if err := s.members.Upsert(ctx, tallerID, u.ID, role, now); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "usuario.agregar", Entity: "usuario",
			EntityID: u.ID, After: map[string]any{"nombre": u.Name, "rol": role}, OccurredAt: now}); err != nil {
			return err
		}
		out, err = s.members.Get(ctx, tallerID, u.ID)
		return err
	})
	return out, err
}

// SetMemberActive desactiva (o reactiva) a un usuario (HU-84). Desactivar revoca sus sesiones de inmediato;
// el token de acceso que ya tenga deja de servir en máximo 10 minutos.
func (s *Service) SetMemberActive(ctx context.Context, tallerID shared.ID, actor shared.Actor, userID shared.ID, active bool) (Member, error) {
	if userID == actor.ID {
		return Member{}, ErrSelf
	}
	now := s.clock.Now()
	var out Member
	err := s.tx.DoWithSettings(ctx, map[string]string{settingTaller: tallerID.String()}, func(ctx context.Context) error {
		m, err := s.members.Get(ctx, tallerID, userID)
		if err != nil {
			return err
		}
		if m.Role == string(domain.RoleOwner) {
			return ErrSelf
		}
		if err := s.members.SetActive(ctx, tallerID, userID, active); err != nil {
			return err
		}
		if !active {
			if err := s.members.RevokeSessions(ctx, tallerID, userID, now, "usuario_desactivado"); err != nil {
				return err
			}
		}
		if err := s.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "usuario.activo", Entity: "usuario",
			EntityID: userID, Before: map[string]any{"activo": m.Active}, After: map[string]any{"activo": active}, OccurredAt: now}); err != nil {
			return err
		}
		out, err = s.members.Get(ctx, tallerID, userID)
		return err
	})
	return out, err
}

// Workshops lista los talleres del usuario (para escoger en cuál trabaja, HU-80).
func (s *Service) Workshops(ctx context.Context, userID shared.ID) ([]ports.Workshop, error) {
	var out []ports.Workshop
	err := s.tx.DoWithSettings(ctx, map[string]string{settingUsuario: userID.String()}, func(ctx context.Context) error {
		var err error
		out, err = s.members.Workshops(ctx, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	// Cada taller solo se deja leer con su propio app.taller_id (RLS): una transacción corta por taller.
	for i := range out {
		id := out[i].ID
		if err := s.tx.DoWithSettings(ctx, map[string]string{settingTaller: id.String()}, func(ctx context.Context) error {
			ws, err := s.workshops.Get(ctx, id)
			out[i].Name = ws.Name
			return err
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Switch abre una sesión en otro taller del mismo usuario y cierra la actual.
func (s *Service) Switch(ctx context.Context, userID, currentSession, tallerID shared.ID) (Session, error) {
	now := s.clock.Now()
	var out Session
	err := s.tx.DoWithSettings(ctx, map[string]string{settingUsuario: userID.String(), settingTaller: tallerID.String()}, func(ctx context.Context) error {
		role, ok, err := s.store.ActiveRole(ctx, tallerID, userID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotMember
		}
		u, err := s.store.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		ws, err := s.workshops.Get(ctx, tallerID)
		if err != nil {
			return err
		}
		if err := s.store.RevokeSession(ctx, currentSession, now, "cambio_de_taller"); err != nil {
			return err
		}
		out, err = s.openSession(ctx, u, ws, []string{role}, now)
		return err
	})
	return out, err
}
