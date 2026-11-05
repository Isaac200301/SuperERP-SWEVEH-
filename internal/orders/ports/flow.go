package ports

import (
	"context"
	"errors"
	"io"
	"time"

	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// FlowRepository persiste estados, checklist, evidencias y lecturas completas de la orden.
type FlowRepository interface {
	Load(ctx context.Context, tallerID, id shared.ID) (domain.Order, error)
	SaveState(ctx context.Context, o domain.Order, prevVersion int) error
	SaveAssignment(ctx context.Context, o domain.Order) error
	// SaveReceipt guarda la respuesta a la constancia solo si no había una (la primera manda, HU-19).
	SaveReceipt(ctx context.Context, o domain.Order) error
	MarkExample(ctx context.Context, tallerID, id shared.ID) error
	InsertHistory(ctx context.Context, tallerID, orderID shared.ID, from *domain.State, to domain.State, by *shared.ID, reason string, at time.Time) error
	List(ctx context.Context, f ListFilter) ([]domain.Order, error)
	// MoveCustomer pasa las órdenes de un cliente a otro (unir clientes repetidos, HU-15).
	MoveCustomer(ctx context.Context, tallerID, from, to shared.ID) (int, error)

	GetChecklist(ctx context.Context, tallerID, orderID shared.ID) (domain.Checklist, bool, error)
	SaveChecklist(ctx context.Context, tallerID shared.ID, c domain.Checklist) error

	InsertEvidence(ctx context.Context, e Evidence) error
	ListEvidence(ctx context.Context, tallerID, orderID shared.ID) ([]Evidence, error)
	GetEvidence(ctx context.Context, tallerID, orderID, id shared.ID) (Evidence, error)
	PhotoSides(ctx context.Context, tallerID, orderID shared.ID) ([]string, error)
	// FindingInOrder dice si el hallazgo es de esa orden (anti-IDOR al adjuntar fotos).
	FindingInOrder(ctx context.Context, tallerID, orderID, findingID shared.ID) (bool, error)
}

// ListFilter filtra la lista de órdenes.
type ListFilter struct {
	TallerID   shared.ID
	OnlyOpen   bool
	State      *domain.State
	VehicleID  *shared.ID
	MechanicID *shared.ID
	Limit      int
	Cursor     *Cursor
}

// Evidence es una foto o nota de la orden (HU-21).
type Evidence struct {
	ID          shared.ID
	TallerID    shared.ID
	OrderID     shared.ID
	Side        *string
	Note        *string
	StorageKey  *string
	ContentType *string
	ShowClient  bool
	Stage       string
	AuthorID    *shared.ID
	CreatedAt   time.Time
	GroupID     *shared.ID // fotos que se subieron juntas con una nota
	FindingID   *shared.ID // foto de un hallazgo del diagnóstico
}

// ErrStorageTooLarge lo devuelve Storage.Put cuando el archivo pasa el tamaño máximo.
var ErrStorageTooLarge = errors.New("archivo demasiado grande")

// Storage guarda los archivos de las fotos.
type Storage interface {
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// QuoteSummary es lo que el historial muestra de la cotización de una orden.
type QuoteSummary struct {
	State string
	Total int64
	Lines []QuoteLine
}

type QuoteLine struct {
	Kind        string
	Description string
	Qty         int
	UnitPrice   int64
}

// QuoteReader lee la cotización de una orden (fachada de quotes).
type QuoteReader interface {
	ForOrder(ctx context.Context, tallerID, orderID shared.ID) (*QuoteSummary, error)
}
