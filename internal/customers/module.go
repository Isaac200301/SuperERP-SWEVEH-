// Package customers es la fachada pública del módulo: lo único que otros módulos pueden importar.
package customers

import (
	"context"

	"sweveh/internal/customers/app"
	"sweveh/internal/shared"
)

// Summary es lo que otros módulos ven de un cliente.
type Summary struct {
	ID      shared.ID
	Name    string
	Phone   string // E.164
	Consent string // pendiente | autorizada | rechazada
}

// Directory es la fachada de lectura para otros módulos.
type Directory struct{ svc *app.Service }

func NewDirectory(svc *app.Service) Directory { return Directory{svc: svc} }

// Exists indica si el cliente pertenece al taller.
func (d Directory) Exists(ctx context.Context, tallerID, id shared.ID) (bool, error) {
	_, err := d.svc.ByID(ctx, tallerID, id)
	if err == nil {
		return true, nil
	}
	if e, ok := shared.AsError(err); ok && e.Kind == shared.KindNotFound {
		return false, nil
	}
	return false, err
}

// Search busca clientes por celular o nombre (HU-14).
func (d Directory) Search(ctx context.Context, tallerID shared.ID, q string) ([]Summary, error) {
	list, err := d.svc.Search(ctx, tallerID, q)
	out := make([]Summary, 0, len(list))
	for _, c := range list {
		out = append(out, Summary{ID: c.ID, Name: c.Name, Phone: c.Phone, Consent: string(c.Consent)})
	}
	return out, err
}

// ByPhone devuelve el cliente con ese celular (E.164) o nil (WhatsApp, HU-51).
func (d Directory) ByPhone(ctx context.Context, tallerID shared.ID, phone string) (*Summary, error) {
	c, err := d.svc.ByPhone(ctx, tallerID, phone)
	if e, ok := shared.AsError(err); ok && e.Kind == shared.KindNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Summary{ID: c.ID, Name: c.Name, Phone: c.Phone, Consent: string(c.Consent)}, nil
}

// Get devuelve un cliente del taller.
func (d Directory) Get(ctx context.Context, tallerID, id shared.ID) (Summary, error) {
	c, err := d.svc.ByID(ctx, tallerID, id)
	return Summary{ID: c.ID, Name: c.Name, Phone: c.Phone, Consent: string(c.Consent)}, err
}

// Register busca o crea un cliente por celular (lo usa el agente al recibir un mensaje).
func (d Directory) Register(ctx context.Context, tallerID shared.ID, phone, name string, actor shared.Actor) (Summary, error) {
	r, err := d.svc.Register(ctx, app.RegisterCustomer{TallerID: tallerID, Actor: actor, Phone: phone, Name: name})
	c := r.Customer
	return Summary{ID: c.ID, Name: c.Name, Phone: c.Phone, Consent: string(c.Consent)}, err
}

// SetConsent registra la autorización del cliente con su prueba (HU-85).
func (d Directory) SetConsent(ctx context.Context, tallerID, id shared.ID, granted bool, rec app.ConsentRecord, actor shared.Actor) error {
	return d.svc.SetConsent(ctx, tallerID, id, actor, granted, rec)
}

// ConsentRecord es la prueba de la autorización (re-exportado para otros módulos).
type ConsentRecord = app.ConsentRecord

// SummariesByIDs devuelve un mapa id → resumen.
func (d Directory) SummariesByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) (map[shared.ID]Summary, error) {
	list, err := d.svc.ByIDs(ctx, tallerID, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[shared.ID]Summary, len(list))
	for _, c := range list {
		out[c.ID] = Summary{ID: c.ID, Name: c.Name, Phone: c.Phone}
	}
	return out, nil
}
