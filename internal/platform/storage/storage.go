// Package storage guarda archivos (fotos de evidencia) detrás de un puerto (docs/06, ObjectStorage).
// Adaptador local en disco; Supabase Storage (bucket privado) llega con la cuenta de staging.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// Store guarda y lee objetos por clave.
type Store interface {
	Put(ctx context.Context, key string, r io.Reader, maxBytes int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

var ErrTooLarge = errors.New("storage: archivo demasiado grande")
var validKey = regexp.MustCompile(`^[a-f0-9-]{36}/[a-f0-9-]{36}$`)

// Local guarda en una carpeta. Las claves son taller/archivo (UUID) para que no haya rutas arbitrarias.
type Local struct{ dir string }

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Local{dir: dir}, nil
}

func (l *Local) path(key string) (string, error) {
	if !validKey.MatchString(key) {
		return "", fmt.Errorf("storage: clave inválida")
	}
	return filepath.Join(l.dir, filepath.FromSlash(key)), nil
}

func (l *Local) Put(_ context.Context, key string, r io.Reader, maxBytes int64) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, maxBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxBytes {
		err = ErrTooLarge
	}
	if err != nil {
		_ = os.Remove(p)
	}
	return err
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p) // #nosec G304 -- la clave se valida con validKey (solo UUIDs)
}
