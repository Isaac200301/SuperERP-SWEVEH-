// Package storage adapta el almacenamiento de plataforma al puerto de orders.
package storage

import (
	"context"
	"errors"
	"io"

	"sweveh/internal/orders/ports"
	"sweveh/internal/platform/storage"
)

type Adapter struct{ s storage.Store }

func New(s storage.Store) Adapter { return Adapter{s: s} }

func (a Adapter) Put(ctx context.Context, key string, r io.Reader, max int64) error {
	err := a.s.Put(ctx, key, r, max)
	if errors.Is(err, storage.ErrTooLarge) {
		return ports.ErrStorageTooLarge
	}
	return err
}

func (a Adapter) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	return a.s.Open(ctx, key)
}
