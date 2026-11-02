// Package postgres implementa los puertos de customers con sqlc y la transacción del context.
package postgres

import (
	"context"

	"sweveh/internal/customers/adapters/out/postgres/sqlcgen"
	"sweveh/internal/customers/domain"
	"sweveh/internal/platform/db"
	"sweveh/internal/shared"
)

// Repository implementa ports.CustomerRepository.
type Repository struct{}

func NewRepository() Repository { return Repository{} }

func queries(ctx context.Context) (*sqlcgen.Queries, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	return sqlcgen.New(conn), nil
}

func (Repository) ByPhone(ctx context.Context, tallerID shared.ID, phone string) (domain.Customer, error) {
	q, err := queries(ctx)
	if err != nil {
		return domain.Customer{}, err
	}
	r, err := q.GetCustomerByPhone(ctx, sqlcgen.GetCustomerByPhoneParams{TallerID: tallerID, Celular: phone})
	if err != nil {
		return domain.Customer{}, db.NotFoundIfNoRows(err, domain.ErrCustomerNotFound)
	}
	return toDomain(sqlcgen.GetCustomerByIDRow(r)), nil
}

func (Repository) ByID(ctx context.Context, tallerID, id shared.ID) (domain.Customer, error) {
	q, err := queries(ctx)
	if err != nil {
		return domain.Customer{}, err
	}
	r, err := q.GetCustomerByID(ctx, sqlcgen.GetCustomerByIDParams{TallerID: tallerID, ID: id})
	if err != nil {
		return domain.Customer{}, db.NotFoundIfNoRows(err, domain.ErrCustomerNotFound)
	}
	return toDomain(r), nil
}

func (Repository) ByIDs(ctx context.Context, tallerID shared.ID, ids []shared.ID) ([]domain.Customer, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q, err := queries(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListCustomersByIDs(ctx, sqlcgen.ListCustomersByIDsParams{TallerID: tallerID, Ids: ids})
	if err != nil {
		return nil, err
	}
	out := make([]domain.Customer, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDomain(sqlcgen.GetCustomerByIDRow(r)))
	}
	return out, nil
}

func (Repository) Insert(ctx context.Context, c domain.Customer, createdBy shared.ID) (bool, error) {
	q, err := queries(ctx)
	if err != nil {
		return false, err
	}
	var by *shared.ID
	if createdBy != shared.NilID {
		by = &createdBy
	}
	n, err := q.InsertCustomer(ctx, sqlcgen.InsertCustomerParams{
		ID: c.ID, TallerID: c.TallerID, Celular: c.Phone, Nombre: c.Name, Correo: c.Email,
		AutorizacionEstado: string(c.Consent), CreatedAt: c.CreatedAt, CreatedBy: by,
	})
	return n == 1, err
}

func toDomain(r sqlcgen.GetCustomerByIDRow) domain.Customer {
	return domain.Customer{
		ID: r.ID, TallerID: r.TallerID, Phone: r.Celular, Name: r.Nombre, Email: r.Correo,
		Consent: domain.ConsentStatus(r.AutorizacionEstado), Version: int(r.Version), CreatedAt: r.CreatedAt,
	}
}
