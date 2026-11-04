// Package photos adapta el almacenamiento de la plataforma a la vitrina del inventario.
package photos

import (
	"context"
	"errors"
	"io"

	"sweveh/internal/inventory/app"
	"sweveh/internal/platform/storage"
)

type Adapter struct{ s storage.Store }

func New(s storage.Store) Adapter { return Adapter{s: s} }

func (a Adapter) Put(ctx context.Context, key string, r io.Reader, max int64) error {
	err := a.s.Put(ctx, key, r, max)
	if errors.Is(err, storage.ErrTooLarge) {
		return app.ErrTooLarge
	}
	return err
}

func (a Adapter) Open(ctx context.Context, key string) (io.ReadCloser, error) { return a.s.Open(ctx, key) }
