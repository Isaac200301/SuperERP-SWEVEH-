// Package orders es la fachada pública del módulo de órdenes.
package orders

import (
	"context"
	"time"

	"sweveh/internal/orders/app"
	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// Summary es lo que otros módulos ven de una orden.
type Summary struct {
	ID          shared.ID
	Number      int
	Code        string
	State       string
	CustomerID  shared.ID
	VehicleID   shared.ID
	Plate       string
	VehicleKind string
	Example     bool
	CreatedAt   time.Time
	ClosedAt    *time.Time
	PromisedAt  *time.Time
}

type Orders struct{ flow *app.Flow }

func New(f *app.Flow) Orders { return Orders{flow: f} }

func summary(o domain.Order) Summary {
	return Summary{ID: o.ID, Number: o.Number, Code: o.Code(), State: string(o.State), CustomerID: o.CustomerID, VehicleID: o.VehicleID,
		Plate: o.Plate, VehicleKind: o.VehicleKind, Example: o.Example, CreatedAt: o.CreatedAt, ClosedAt: o.ClosedAt, PromisedAt: o.PromisedAt}
}

// Load lee la orden dentro de la transacción del llamador.
func (o Orders) Load(ctx context.Context, tallerID, id shared.ID) (Summary, error) {
	x, err := o.flow.Load(ctx, tallerID, id)
	return summary(x), err
}

// Move cambia el estado por una causa del sistema (cotización enviada o aprobada).
func (o Orders) Move(ctx context.Context, tallerID, id shared.ID, to string, actor shared.Actor, reason string) error {
	_, err := o.flow.SystemTransition(ctx, tallerID, id, domain.State(to), actor, reason)
	return err
}

// OpenByVehicle devuelve las órdenes abiertas de un vehículo (agente, búsqueda).
// AnswerReceipt registra "Está bien" (ok) o "Algo no cuadra" a la constancia de recibo (HU-19).
func (o Orders) AnswerReceipt(ctx context.Context, tallerID, id shared.ID, actor shared.Actor, ok bool, ref string) (bool, error) {
	return o.flow.AnswerReceipt(ctx, tallerID, id, actor, ok, ref)
}

func (o Orders) OpenByVehicle(ctx context.Context, tallerID, vehicleID shared.ID) ([]Summary, error) {
	views, err := o.flow.ListIn(ctx, tallerID, &vehicleID, true)
	out := make([]Summary, 0, len(views))
	for _, v := range views {
		out = append(out, summary(v))
	}
	return out, err
}

// Info es la orden con checklist y antigüedad en su estado (avisos AU-01, AU-03, AU-07).
type Info struct {
	Summary
	Vehicle     string
	Km          *int
	Exceptions  []string
	Accessories []string
	StateSince  time.Time
	// Receipt es la respuesta a la constancia: "", conforme u observado (HU-19).
	Receipt string
}

func info(i app.Info) Info {
	return Info{Summary: summary(i.Order), Vehicle: i.Vehicle, Km: i.Km, Exceptions: i.Exceptions, Accessories: i.Accessories,
		StateSince: i.StateSince, Receipt: i.Order.ReceiptAnswer}
}

// ObservedReceipts son las órdenes abiertas cuyo cliente dijo "Algo no cuadra" en la constancia (HU-19).
func (o Orders) ObservedReceipts(ctx context.Context, tallerID shared.ID) ([]Info, error) {
	var out []Info
	for _, st := range []domain.State{domain.StateReceived, domain.StateDiagnosis, domain.StateAwaitingOK, domain.StateRepair,
		domain.StateAwaitingParts, domain.StateQuality, domain.StateReady} {
		list, err := o.InState(ctx, tallerID, string(st))
		if err != nil {
			return nil, err
		}
		for _, i := range list {
			if i.Receipt == domain.ReceiptObserved {
				out = append(out, i)
			}
		}
	}
	return out, nil
}

func (o Orders) Info(ctx context.Context, tallerID, id shared.ID) (Info, error) {
	i, err := o.flow.Info(ctx, tallerID, id)
	return info(i), err
}

func (o Orders) InState(ctx context.Context, tallerID shared.ID, state string) ([]Info, error) {
	list, err := o.flow.InState(ctx, tallerID, state)
	out := make([]Info, 0, len(list))
	for _, i := range list {
		out = append(out, info(i))
	}
	return out, err
}

// Stats da las cifras para los KPIs (dentro de la transacción del llamador).
func (o Orders) Stats() app.Stats { return o.flow.Stats() }

// OpenByCustomer lista las órdenes abiertas de un cliente (agente: "¿ya está mi carro?").
func (o Orders) OpenByCustomer(ctx context.Context, tallerID, customerID shared.ID, vehicleIDs []shared.ID) ([]Info, error) {
	var out []Info
	for _, vid := range vehicleIDs {
		v := vid
		list, err := o.flow.ListIn(ctx, tallerID, &v, true)
		if err != nil {
			return nil, err
		}
		for _, x := range list {
			if x.CustomerID != customerID {
				continue // dueño anterior del vehículo: no ve sus órdenes (HU-12)
			}
			i, err := o.flow.Info(ctx, tallerID, x.ID)
			if err != nil {
				return nil, err
			}
			out = append(out, info(i))
		}
	}
	return out, nil
}

// LastShownPhoto devuelve la última foto marcada "mostrar al cliente" (AU-05, HU-21).
func (o Orders) LastShownPhoto(ctx context.Context, tallerID, orderID shared.ID) (*shared.ID, error) {
	return o.flow.LastShownPhoto(ctx, tallerID, orderID)
}
