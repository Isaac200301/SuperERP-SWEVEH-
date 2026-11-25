package app

import (
	"context"
	"regexp"
	"time"

	"sweveh/internal/quotes/domain"
	"sweveh/internal/shared"
)

// V-10 (ADR-0043): la cotización se muestra al cliente en la pantalla del taller y se comparte en PDF. Las dos
// salidas se arman con ClientDoc, que solo trae lo que el cliente puede ver.

// OrderCard es lo que la vista del cliente necesita de la orden y su vehículo.
type OrderCard struct {
	Code       string // OS-0044
	Plate      string
	Vehicle    string // "Mazda 2", "moto"
	CustomerID shared.ID
}

// CustomerCard es lo que se necesita del cliente: el nombre para el encabezado y si ya autorizó sus datos.
type CustomerCard struct {
	Name    string
	Consent string // pendiente | autorizada | rechazada | revocada
}

// Directory es lo que la vista del cliente lee de otros módulos (cada uno por su fachada).
type Directory interface {
	WorkshopName(ctx context.Context, tallerID shared.ID) (string, error)
	Order(ctx context.Context, tallerID, orderID shared.ID) (OrderCard, error)
	Customer(ctx context.Context, tallerID, customerID shared.ID) (CustomerCard, error)
}

// Renderer escribe el PDF de la cotización a partir de ClientDoc.
type Renderer interface {
	Render(doc ClientDoc) ([]byte, error)
}

// ClientDoc es el documento que ve el cliente: el taller, el vehículo y la cotización. Sin costos internos, gastos
// ni notas internas.
type ClientDoc struct {
	Workshop     string
	OrderCode    string
	Plate        string
	Vehicle      string
	CustomerName string
	Quote        domain.ClientView
	// At es la hora de la consulta; el PDF la usa si la cotización aún no se ha enviado.
	At time.Time
}

// Authorization dice si el cliente ya autorizó sus datos; la pantalla lo pide antes de registrar su aprobación en
// el mostrador (HU-86). No es parte del documento del cliente.
type Authorization struct {
	CustomerID shared.ID
	State      string
}

// ClientScreen es la respuesta de la pantalla del cliente: el documento y lo que el taller necesita para actuar.
type ClientScreen struct {
	Doc  ClientDoc
	Auth Authorization
}

// SetDirectory conecta las lecturas de otros módulos y el escritor de PDF (V-10).
func (s *Quotes) SetDirectory(d Directory, r Renderer) { s.dir, s.pdf = d, r }

// ClientView arma lo que se le muestra al cliente de la cotización vigente de la orden. Sirve en cualquier estado:
// un borrador se puede mostrar o compartir, pero solo una enviada se aprueba.
func (s *Quotes) ClientView(ctx context.Context, tallerID, orderID shared.ID) (ClientScreen, error) {
	var out ClientScreen
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		q, ok, err := s.repo.Latest(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		if !ok {
			if _, err := s.orders.Load(ctx, tallerID, orderID); err != nil {
				return err // la orden no existe en este taller
			}
			return domain.ErrNotFound
		}
		o, err := s.dir.Order(ctx, tallerID, orderID)
		if err != nil {
			return err
		}
		name, err := s.dir.WorkshopName(ctx, tallerID)
		if err != nil {
			return err
		}
		c, err := s.dir.Customer(ctx, tallerID, o.CustomerID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		out = ClientScreen{
			Doc: ClientDoc{Workshop: name, OrderCode: o.Code, Plate: o.Plate, Vehicle: o.Vehicle, CustomerName: c.Name, Quote: q.ClientView(now),
				At: now},
			Auth: Authorization{CustomerID: o.CustomerID, State: c.Consent},
		}
		return nil
	})
	return out, err
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9-]+`)

// PDF escribe la cotización en PDF carta (HU-109). Devuelve también el nombre del archivo.
func (s *Quotes) PDF(ctx context.Context, tallerID, orderID shared.ID) (body []byte, filename string, err error) {
	screen, err := s.ClientView(ctx, tallerID, orderID)
	if err != nil {
		return nil, "", err
	}
	body, err = s.pdf.Render(screen.Doc)
	if err != nil {
		return nil, "", err
	}
	return body, "cotizacion-" + unsafeName.ReplaceAllString(screen.Doc.OrderCode, "") + ".pdf", nil
}
