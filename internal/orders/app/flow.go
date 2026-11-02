package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"time"

	"sweveh/internal/audit"
	"sweveh/internal/orders/domain"
	"sweveh/internal/orders/ports"
	"sweveh/internal/shared"
)

// Flow agrupa los casos de uso del ciclo de la orden (HU-17 a HU-26).
type Flow struct {
	uow      ports.UnitOfWork
	repo     ports.FlowRepository
	base     *Service
	vehicles ports.VehicleDirectory
	storage  ports.Storage
	quotes   ports.QuoteReader
	audit    ports.AuditLog
	events   ports.EventPublisher
	clock    ports.Clock
	stats    Stats
}

func portsFilter(tallerID shared.ID, st *domain.State) ports.ListFilter {
	return ports.ListFilter{TallerID: tallerID, State: st, Limit: 100}
}

func NewFlow(uow ports.UnitOfWork, repo ports.FlowRepository, base *Service, v ports.VehicleDirectory, st ports.Storage,
	a ports.AuditLog, e ports.EventPublisher, c ports.Clock) *Flow {
	return &Flow{uow: uow, repo: repo, base: base, vehicles: v, storage: st, audit: a, events: e, clock: c}
}

// SetQuoteReader rompe la dependencia circular de construcción (quotes también usa la fachada de orders).
func (f *Flow) SetQuoteReader(q ports.QuoteReader) { f.quotes = q }

const MaxPhotoBytes = 8 << 20

var (
	ErrPhotoType  = shared.Validation("foto_tipo", "La foto debe ser JPG, PNG o WEBP.").WithField("foto")
	ErrPhotoSize  = shared.Validation("foto_grande", "La foto pesa más de 8 MB.").WithField("foto")
	ErrEvidence   = shared.NotFound("evidencia_no_encontrada", "No encontramos esa foto en la orden.")
	ErrEmptyEvid  = shared.Validation("evidencia_vacia", "Agrega una foto o escribe una nota.")
)

// TransitionCmd es un cambio de estado pedido por una persona.
type TransitionCmd struct {
	TallerID, OrderID shared.ID
	Actor             shared.Actor
	To                domain.State
	Version           int
	CanOverride       bool
	Reason            string
	PickedUpBy        string
	AccessoriesOK     bool
}

// Transition mueve la orden (HU-20) con historial inmutable, auditoría y evento en la misma transacción.
func (f *Flow) Transition(ctx context.Context, c TransitionCmd) (domain.Order, error) {
	return f.move(ctx, c, false)
}

// SystemTransition la usa la cotización (enviada → Esperando aprobación; aprobada → En reparación).
func (f *Flow) SystemTransition(ctx context.Context, tallerID, orderID shared.ID, to domain.State, actor shared.Actor, reason string) (domain.Order, error) {
	return f.move(ctx, TransitionCmd{TallerID: tallerID, OrderID: orderID, Actor: actor, To: to, Reason: reason}, true)
}

func (f *Flow) move(ctx context.Context, c TransitionCmd, fromQuote bool) (domain.Order, error) {
	var out domain.Order
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, c.TallerID, c.OrderID)
		if err != nil {
			return err
		}
		in := domain.TransitionInput{To: c.To, FromQuote: fromQuote, CanOverride: c.CanOverride, Reason: c.Reason,
			PickedUpBy: c.PickedUpBy, AccessoriesOK: c.AccessoriesOK, ExpectedVersion: c.Version, At: f.clock.Now()}
		// La puerta de Recibido aplica al avanzar desde Por recibir, no al devolverse desde En diagnóstico.
		if c.To == domain.StateReceived && o.State == domain.StatePending {
			v, err := f.vehicles.ByID(ctx, c.TallerID, o.VehicleID)
			if err != nil {
				return err
			}
			in.EnginePending = v.EnginePending
			cl, _, err := f.repo.GetChecklist(ctx, c.TallerID, o.ID)
			if err != nil {
				return err
			}
			sides, err := f.repo.PhotoSides(ctx, c.TallerID, o.ID)
			if err != nil {
				return err
			}
			cl.Kind = o.VehicleKind
			missing := cl.Missing(sides)
			in.ChecklistComplete = len(missing) == 0
			if !in.ChecklistComplete {
				return domain.ErrChecklist.WithMessage("Para pasar a Recibido falta: " + join(missing) + ".")
			}
		}
		prevVersion := o.Version
		prev, err := o.Transition(in)
		if err != nil {
			return err
		}
		return f.persistMove(ctx, &o, prev, prevVersion, c.Actor, c.Reason, &out)
	})
	return out, err
}

func (f *Flow) persistMove(ctx context.Context, o *domain.Order, prev domain.State, prevVersion int, actor shared.Actor, reason string, out *domain.Order) error {
	now := f.clock.Now()
	if o.State == domain.StateReceived && prev == domain.StatePending {
		if err := f.completeChecklist(ctx, o, actor, now); err != nil {
			return err
		}
	}
	if err := f.repo.SaveState(ctx, *o, prevVersion); err != nil {
		return err
	}
	if err := f.repo.InsertHistory(ctx, o.TallerID, o.ID, &prev, o.State, actorID(actor), reason, now); err != nil {
		return err
	}
	if err := f.audit.Record(ctx, audit.Entry{TallerID: o.TallerID, Actor: actor, Action: "orden.estado", Entity: "orden",
		EntityID: o.ID, Before: map[string]any{"estado": string(prev)},
		After: map[string]any{"estado": string(o.State), "motivo": reason, "recogido_por": o.PickedUpBy}, OccurredAt: now}); err != nil {
		return err
	}
	if err := f.events.Publish(ctx, o.TallerID, shared.Event{Type: "orders.estado_cambiado", AggregateID: o.ID, OccurredAt: now,
		Payload: map[string]any{"desde": string(prev), "hacia": string(o.State), "cliente_id": o.CustomerID.String(),
			"vehiculo_id": o.VehicleID.String(), "placa": o.Plate, "tipo": o.VehicleKind, "numero": o.Number, "ejemplo": o.Example}}); err != nil {
		return err
	}
	*out = *o
	return nil
}

func (f *Flow) completeChecklist(ctx context.Context, o *domain.Order, actor shared.Actor, now time.Time) error {
	cl, _, err := f.repo.GetChecklist(ctx, o.TallerID, o.ID)
	if err != nil {
		return err
	}
	cl.OrderID, cl.Kind, cl.DoneBy, cl.DoneAt = o.ID, o.VehicleKind, actorID(actor), &now
	return f.repo.SaveChecklist(ctx, o.TallerID, cl)
}

// Cancel cancela con motivo (HU-26).
func (f *Flow) Cancel(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor, reason string, version int) (domain.Order, error) {
	var out domain.Order
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		pv := o.Version
		prev, err := o.Cancel(reason, version, f.clock.Now())
		if err != nil {
			return err
		}
		return f.persistMove(ctx, &o, prev, pv, actor, reason, &out)
	})
	return out, err
}

// Assign pone mecánico y fecha prometida (HU-22).
func (f *Flow) Assign(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor, mechanic *shared.ID, promised *time.Time) (domain.Order, error) {
	var out domain.Order
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		before := map[string]any{"mecanico_id": o.MechanicID, "fecha_prometida": o.PromisedAt}
		o.MechanicID, o.PromisedAt = mechanic, promised
		if err := f.repo.SaveAssignment(ctx, o); err != nil {
			return err
		}
		out = o
		return f.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "orden.asignar", Entity: "orden", EntityID: o.ID,
			Before: before, After: map[string]any{"mecanico_id": mechanic, "fecha_prometida": promised}, OccurredAt: f.clock.Now()})
	})
	return out, err
}

// ChecklistView es el checklist con lo que falta.
type ChecklistView struct {
	Checklist domain.Checklist
	Sides     []string
	Missing   []string
}

func (f *Flow) GetChecklist(ctx context.Context, tallerID, orderID shared.ID) (ChecklistView, error) {
	var out ChecklistView
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		cl, _, err := f.repo.GetChecklist(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		cl.OrderID, cl.Kind = o.ID, o.VehicleKind
		sides, err := f.repo.PhotoSides(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		out = ChecklistView{Checklist: cl, Sides: sides, Missing: cl.Missing(sides)}
		return nil
	})
	return out, err
}

// SaveChecklist guarda kilometraje, combustible, accesorios y salvedades (HU-17, HU-18).
func (f *Flow) SaveChecklist(ctx context.Context, tallerID, orderID shared.ID, actor shared.Actor, in domain.Checklist) (ChecklistView, error) {
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !o.State.Open() {
			return domain.ErrClosed
		}
		cur, _, err := f.repo.GetChecklist(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		in.OrderID, in.Kind, in.DoneBy, in.DoneAt = o.ID, o.VehicleKind, cur.DoneBy, cur.DoneAt
		now := f.clock.Now()
		for i := range in.Exceptions {
			if in.Exceptions[i].At.IsZero() {
				in.Exceptions[i].At = now
			}
		}
		if err := in.Validate(); err != nil {
			return err
		}
		if err := f.repo.SaveChecklist(ctx, tallerID, in); err != nil {
			return err
		}
		return f.audit.Record(ctx, audit.Entry{TallerID: tallerID, Actor: actor, Action: "orden.checklist", Entity: "orden", EntityID: o.ID,
			After: map[string]any{"kilometraje": in.Km, "combustible": in.Fuel, "accesorios": in.Accessories, "salvedades": len(in.Exceptions)},
			OccurredAt: now})
	})
	if err != nil {
		return ChecklistView{}, err
	}
	return f.GetChecklist(ctx, tallerID, orderID)
}

// EvidenceInput es una foto y/o nota nueva.
type EvidenceInput struct {
	TallerID, OrderID shared.ID
	Actor             shared.Actor
	Side              *string
	Note              *string
	ShowClient        bool
	Photo             io.Reader // nil si es solo nota
}

// AddEvidence guarda foto (validada por contenido) y/o nota con hora, autor y etapa (HU-21).
// La foto se sube antes de abrir la transacción: nunca I/O lento dentro de ella.
func (f *Flow) AddEvidence(ctx context.Context, in EvidenceInput) (ports.Evidence, error) {
	e := ports.Evidence{ID: shared.NewID(), TallerID: in.TallerID, OrderID: in.OrderID, Side: in.Side, Note: in.Note,
		ShowClient: in.ShowClient, AuthorID: actorID(in.Actor), CreatedAt: f.clock.Now()}
	if in.Side != nil && !slices.Contains(domain.MotoSides, *in.Side) && *in.Side != "otro" {
		return e, shared.Validation("lado_invalido", "Ese lado no existe.").WithField("lado")
	}
	if in.Photo == nil && (in.Note == nil || *in.Note == "") {
		return e, ErrEmptyEvid
	}
	// Antes de subir, confirmar que la orden es del taller (anti-IDOR).
	var stage domain.State
	if err := f.uow.Do(ctx, func(ctx context.Context) error {
		o, err := f.repo.Load(ctx, in.TallerID, in.OrderID)
		stage = o.State
		return err
	}); err != nil {
		return e, err
	}
	e.Stage = string(stage)
	if in.Photo != nil {
		head := make([]byte, 12)
		n, _ := io.ReadFull(in.Photo, head)
		ct := sniffImage(head[:n])
		if ct == "" {
			return e, ErrPhotoType
		}
		key := in.TallerID.String() + "/" + e.ID.String()
		if err := f.storage.Put(ctx, key, io.MultiReader(bytes.NewReader(head[:n]), in.Photo), MaxPhotoBytes); err != nil {
			if errors.Is(err, ports.ErrStorageTooLarge) {
				return e, ErrPhotoSize
			}
			return e, err
		}
		e.StorageKey, e.ContentType = &key, &ct
	}
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		if err := f.repo.InsertEvidence(ctx, e); err != nil {
			return err
		}
		return f.audit.Record(ctx, audit.Entry{TallerID: in.TallerID, Actor: in.Actor, Action: "orden.evidencia", Entity: "orden",
			EntityID: in.OrderID, After: map[string]any{"evidencia_id": e.ID.String(), "lado": in.Side, "foto": in.Photo != nil,
				"mostrar_cliente": in.ShowClient}, OccurredAt: e.CreatedAt})
	})
	return e, err
}

func (f *Flow) ListEvidence(ctx context.Context, tallerID, orderID shared.ID) ([]ports.Evidence, error) {
	var out []ports.Evidence
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		if _, err := f.repo.Load(ctx, tallerID, orderID); err != nil {
			return err
		}
		var err error
		out, err = f.repo.ListEvidence(ctx, tallerID, orderID)
		return err
	})
	return out, err
}

// OpenPhoto abre el archivo de una foto de la orden (el handler lo transmite; el bucket es privado).
func (f *Flow) OpenPhoto(ctx context.Context, tallerID, orderID, id shared.ID) (io.ReadCloser, string, error) {
	var e ports.Evidence
	if err := f.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		e, err = f.repo.GetEvidence(ctx, tallerID, orderID, id)
		return err
	}); err != nil {
		return nil, "", err
	}
	if e.StorageKey == nil {
		return nil, "", ErrEvidence
	}
	rc, err := f.storage.Open(ctx, *e.StorageKey)
	return rc, *e.ContentType, err
}

// List devuelve órdenes con filtros, con cliente y vehículo.
func (f *Flow) List(ctx context.Context, fl ports.ListFilter) ([]OrderView, error) {
	var out []OrderView
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		list, err := f.repo.List(ctx, fl)
		if err != nil {
			return err
		}
		out, err = f.base.decorate(ctx, fl.TallerID, list)
		return err
	})
	return out, err
}

// Mine es la lista del piso (HU-23): primero las del mecánico por fecha prometida; si el taller no
// asigna órdenes, todas las abiertas (Q-14).
func (f *Flow) Mine(ctx context.Context, tallerID, userID shared.ID) ([]OrderView, error) {
	mine, err := f.List(ctx, ports.ListFilter{TallerID: tallerID, OnlyOpen: true, MechanicID: &userID, Limit: 100})
	if err != nil || len(mine) > 0 {
		return mine, err
	}
	return f.List(ctx, ports.ListFilter{TallerID: tallerID, OnlyOpen: true, Limit: 100})
}

// HistoryItem es una orden en la ficha del vehículo (HU-13).
type HistoryItem struct {
	View     OrderView
	Evidence []ports.Evidence
	Quote    *ports.QuoteSummary
}

// VehicleHistory devuelve todo lo que se le ha hecho al vehículo (HU-13).
func (f *Flow) VehicleHistory(ctx context.Context, tallerID, vehicleID shared.ID) ([]HistoryItem, error) {
	var out []HistoryItem
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		list, err := f.repo.List(ctx, ports.ListFilter{TallerID: tallerID, VehicleID: &vehicleID, Limit: 100})
		if err != nil {
			return err
		}
		views, err := f.base.decorate(ctx, tallerID, list)
		if err != nil {
			return err
		}
		for _, v := range views {
			ev, err := f.repo.ListEvidence(ctx, tallerID, v.Order.ID)
			if err != nil {
				return err
			}
			item := HistoryItem{View: v, Evidence: ev}
			if f.quotes != nil {
				if item.Quote, err = f.quotes.ForOrder(ctx, tallerID, v.Order.ID); err != nil {
					return err
				}
			}
			out = append(out, item)
		}
		return nil
	})
	return out, err
}

// ListIn lista órdenes de un vehículo dentro de la transacción del llamador.
func (f *Flow) ListIn(ctx context.Context, tallerID shared.ID, vehicleID *shared.ID, onlyOpen bool) ([]domain.Order, error) {
	return f.repo.List(ctx, ports.ListFilter{TallerID: tallerID, VehicleID: vehicleID, OnlyOpen: onlyOpen, Limit: 50})
}

// LastShownPhoto es la última foto marcada para el cliente (AU-05).
func (f *Flow) LastShownPhoto(ctx context.Context, tallerID, orderID shared.ID) (*shared.ID, error) {
	ev, err := f.repo.ListEvidence(ctx, tallerID, orderID)
	if err != nil {
		return nil, err
	}
	for i := len(ev) - 1; i >= 0; i-- {
		if ev[i].ShowClient && ev[i].StorageKey != nil {
			id := ev[i].ID
			return &id, nil
		}
	}
	return nil, nil
}

// Load lee la orden completa (la usan otras fachadas dentro de su transacción).
func (f *Flow) Load(ctx context.Context, tallerID, id shared.ID) (domain.Order, error) {
	return f.repo.Load(ctx, tallerID, id)
}

// sniffImage reconoce JPG, PNG y WEBP por sus bytes iniciales (no se confía en la extensión).
func sniffImage(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	}
	return ""
}

func actorID(a shared.Actor) *shared.ID {
	if a.ID == shared.NilID {
		return nil
	}
	id := a.ID
	return &id
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += "; "
		}
		out += x
	}
	return out
}
