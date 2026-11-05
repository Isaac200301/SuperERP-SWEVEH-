package wire

import (
	"context"
	"errors"
	"fmt"

	automationsapp "sweveh/internal/automations/app"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
	"sweveh/internal/orders"
	"sweveh/internal/server"
	"sweveh/internal/shared"
)

type autoOrders struct{ m *server.Modules }

func orderInfo(i orders.Info) automationsapp.OrderInfo {
	return automationsapp.OrderInfo{ID: i.ID, Code: i.Code, State: i.State, Plate: i.Plate, Vehicle: i.Vehicle, Kind: i.VehicleKind, CustomerID: i.CustomerID,
		PromisedAt: i.PromisedAt, Example: i.Example, Km: i.Km, Exceptions: i.Exceptions, Accessories: i.Accessories, StateSince: i.StateSince}
}

func (a autoOrders) Info(ctx context.Context, tallerID, id shared.ID) (automationsapp.OrderInfo, error) {
	i, err := a.m.OrdersF.Info(ctx, tallerID, id)
	return orderInfo(i), err
}

func (a autoOrders) InState(ctx context.Context, tallerID shared.ID, state string) ([]automationsapp.OrderInfo, error) {
	list, err := a.m.OrdersF.InState(ctx, tallerID, state)
	out := make([]automationsapp.OrderInfo, 0, len(list))
	for _, i := range list {
		out = append(out, orderInfo(i))
	}
	return out, err
}

type autoCustomers struct{ m *server.Modules }

func (a autoCustomers) Get(ctx context.Context, tallerID, id shared.ID) (automationsapp.Customer, error) {
	c, err := a.m.CustDir.Get(ctx, tallerID, id)
	return automationsapp.Customer{Name: c.Name, Phone: c.Phone, Consent: c.Consent}, err
}

type autoWorkshops struct{ m *server.Modules }

func (a autoWorkshops) Get(ctx context.Context, tallerID shared.ID) (automationsapp.Workshop, error) {
	s, err := a.m.Admin.LoadIn(ctx, tallerID)
	w := automationsapp.Workshop{Name: s.Name, Address: s.Profile.Address, Schedule: map[string]*[2]string{}}
	for d, sl := range s.Schedule {
		if sl != nil {
			w.Schedule[d] = &[2]string{sl.Opens, sl.Closes}
		}
	}
	return w, err
}

type autoQuotes struct{ m *server.Modules }

func (a autoQuotes) ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*automationsapp.QuoteInfo, error) {
	q, err := a.m.QuotesF.ForOrder(ctx, tallerID, orderID)
	if err != nil || q == nil {
		return nil, err
	}
	out := &automationsapp.QuoteInfo{Total: q.Total}
	for _, l := range q.Lines {
		out.Lines = append(out.Lines, fmt.Sprintf("%s x%d: %s", l.Description, l.Qty, automationsapp.Money(int64(l.Qty)*l.UnitPrice)))
	}
	return out, nil
}

type autoMessenger struct{ s *Services }

// Send encola por WhatsApp; si la ventana de 24 h está cerrada lo informa en vez de fallar (HU-47).
func (a autoMessenger) Send(ctx context.Context, o automationsapp.Outgoing) (*shared.ID, bool, error) {
	btns := make([]domain.Button, 0, len(o.Buttons))
	for _, b := range o.Buttons {
		btns = append(btns, domain.Button{ID: b.ID, Title: b.Title})
	}
	auto := o.Automation
	m, err := a.s.Messaging.Enqueue(ctx, messagingapp.Outgoing{TallerID: o.TallerID, Phone: o.Phone, Text: o.Text, Buttons: btns,
		Origin: "automatizacion", OrderID: o.OrderID, Automation: &auto})
	if errors.Is(err, messagingapp.ErrOutsideWindow) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &m.ID, false, nil
}
