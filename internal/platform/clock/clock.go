// Package clock da la hora por inyección, para que dominio y casos de uso
// sean deterministas en pruebas (docs/06, puerto Clock).
package clock

import (
	"sync"
	"time"
)

// Clock devuelve la hora actual en UTC.
type Clock interface{ Now() time.Time }

// System es el reloj real.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fake es un reloj controlable para pruebas.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
