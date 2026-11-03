package tenancy

import (
	"context"
	"encoding/json"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
	"sweveh/internal/tenancy/domain"
)

// Preloader carga los servicios iniciales (fachada de catalog).
type Preloader interface {
	Preload(ctx context.Context, tallerID shared.ID, attends string, specialties []string) error
}

type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

// Settings es la configuración del taller: perfil, horario y contrato.
type Settings struct {
	ID                 shared.ID
	Name               string
	Profile            domain.Profile
	Schedule           domain.Schedule
	ContractVersion    *string
	ContractAcceptedAt *time.Time
	Onboarded          bool
	Version            int
}

type settingsRow struct {
	ID         shared.ID       `db:"id"`
	Nombre     string          `db:"nombre"`
	Atiende    *string         `db:"atiende"`
	Especial   []string        `db:"especialidades"`
	Ciudad     *string         `db:"ciudad"`
	Tamano     *string         `db:"tamano"`
	Horario    json.RawMessage `db:"horario"`
	ContratoV  *string         `db:"contrato_version"`
	ContratoEn *time.Time      `db:"contrato_aceptado_en"`
	Onboarding bool            `db:"onboarding_completo"`
	Version    int             `db:"version"`
}

// Admin agrupa los casos de uso de configuración del taller.
type Admin struct {
	uow     UnitOfWork
	catalog Preloader
	audit   AuditLog
	clock   interface{ Now() time.Time }
}

func NewAdmin(uow UnitOfWork, p Preloader, a AuditLog, c interface{ Now() time.Time }) *Admin {
	return &Admin{uow: uow, catalog: p, audit: a, clock: c}
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (a *Admin) load(ctx context.Context, id shared.ID) (Settings, error) {
	r, err := db.One[settingsRow](ctx, domain.ErrWorkshopNotFound, `SELECT id, nombre, atiende, especialidades, ciudad, tamano, horario,
		contrato_version, contrato_aceptado_en, onboarding_completo, version FROM tenancy.taller WHERE id = $1`, id)
	if err != nil {
		return Settings{}, err
	}
	s := Settings{ID: r.ID, Name: r.Nombre, Profile: domain.Profile{Attends: str(r.Atiende), Specialties: r.Especial, City: str(r.Ciudad),
		Size: str(r.Tamano)}, ContractVersion: r.ContratoV, ContractAcceptedAt: r.ContratoEn, Onboarded: r.Onboarding, Version: r.Version}
	if err := json.Unmarshal(r.Horario, &s.Schedule); err != nil {
		return s, err
	}
	return s, nil
}

// Get devuelve la configuración del taller de la sesión.
func (a *Admin) Get(ctx context.Context, id shared.ID) (Settings, error) {
	var out Settings
	err := a.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = a.load(ctx, id)
		return err
	})
	return out, err
}

// LoadIn lee la configuración dentro de la transacción del llamador (agente, avisos).
func (a *Admin) LoadIn(ctx context.Context, id shared.ID) (Settings, error) { return a.load(ctx, id) }

// OnboardInput es el paso 2 del registro (HU-03) más la aceptación del contrato (HU-89).
type OnboardInput struct {
	Name             string
	Profile          domain.Profile
	ContractAccepted bool
}

// Onboard guarda el tipo de taller, acepta el contrato y deja el taller armado (HU-04) en una transacción.
func (a *Admin) Onboard(ctx context.Context, id shared.ID, actor shared.Actor, in OnboardInput) (Settings, error) {
	if err := in.Profile.Validate(); err != nil {
		return Settings{}, err
	}
	if !in.ContractAccepted {
		return Settings{}, domain.ErrContract
	}
	var out Settings
	err := a.uow.Do(ctx, func(ctx context.Context) error {
		cur, err := a.load(ctx, id)
		if err != nil {
			return err
		}
		name := cur.Name
		if in.Name != "" {
			ws, err := domain.NewWorkshop(id, in.Name, a.clock.Now())
			if err != nil {
				return err
			}
			name = ws.Name
		}
		now := a.clock.Now()
		ver := domain.ContractVersionDraft
		if _, err := db.Exec(ctx, `UPDATE tenancy.taller SET nombre = $2, atiende = $3, especialidades = $4, ciudad = $5, tamano = $6,
			contrato_version = COALESCE(contrato_version, $7), contrato_aceptado_en = COALESCE(contrato_aceptado_en, $8),
			onboarding_completo = true, version = version + 1, updated_at = now() WHERE id = $1`,
			id, name, in.Profile.Attends, in.Profile.Specialties, nilIfEmpty(in.Profile.City), in.Profile.Size, ver, now); err != nil {
			return err
		}
		if err := a.catalog.Preload(ctx, id, in.Profile.Attends, in.Profile.Specialties); err != nil {
			return err
		}
		if err := a.audit.Record(ctx, audit.Entry{TallerID: id, Actor: actor, Action: "taller.configurar", Entity: "taller", EntityID: id,
			Before: map[string]any{"atiende": cur.Profile.Attends}, After: map[string]any{"nombre": name, "atiende": in.Profile.Attends,
				"tamano": in.Profile.Size, "contrato": ver}, OccurredAt: now}); err != nil {
			return err
		}
		out, err = a.load(ctx, id)
		return err
	})
	return out, err
}

// SetSchedule guarda el horario (HU-08).
func (a *Admin) SetSchedule(ctx context.Context, id shared.ID, actor shared.Actor, s domain.Schedule) (Settings, error) {
	if err := s.Validate(); err != nil {
		return Settings{}, err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return Settings{}, err
	}
	var out Settings
	err = a.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := db.Exec(ctx, `UPDATE tenancy.taller SET horario = $2, version = version + 1, updated_at = now() WHERE id = $1`, id, raw); err != nil {
			return err
		}
		if err := a.audit.Record(ctx, audit.Entry{TallerID: id, Actor: actor, Action: "taller.horario", Entity: "taller", EntityID: id,
			After: map[string]any{"horario": s}, OccurredAt: a.clock.Now()}); err != nil {
			return err
		}
		var err error
		out, err = a.load(ctx, id)
		return err
	})
	return out, err
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
