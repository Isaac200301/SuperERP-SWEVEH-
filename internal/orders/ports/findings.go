package ports

import (
	"context"

	"sweveh/internal/orders/domain"
	"sweveh/internal/shared"
)

// FindingRepository guarda los hallazgos del diagnóstico (HU-103 propuesta).
type FindingRepository interface {
	InsertFinding(ctx context.Context, f domain.Finding) error
	ListFindings(ctx context.Context, tallerID, orderID shared.ID) ([]domain.Finding, error)
	DeleteFinding(ctx context.Context, tallerID, orderID, id shared.ID) (domain.Finding, error)
}
