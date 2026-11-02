// Package otp contiene los adaptadores de entrega del código. El canal real (SMS o plantilla
// de autenticación de WhatsApp) sigue abierto (Q-13/AQ-03) y su costo es `Sin dato`.
package otp

import (
	"context"
	"fmt"
	"io"
	"sync"

	"sweveh/internal/platform/logging"
)

// Console escribe el código en la consola. SOLO para APP_ENV=local: la configuración
// rechaza este adaptador en cualquier otro entorno, porque el OTP nunca va a logs (docs/08 §6).
type Console struct {
	mu  sync.Mutex
	out io.Writer
}

func NewConsole(out io.Writer) *Console { return &Console{out: out} }

func (c *Console) Send(_ context.Context, phone, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := fmt.Fprintf(c.out, "\n[SOLO DESARROLLO] Código SWEVEH para %s: %s\n\n", logging.MaskPhone(phone), code)
	return err
}

// Memory guarda el último código por celular. Para pruebas automáticas.
type Memory struct {
	mu    sync.Mutex
	codes map[string]string
}

func NewMemory() *Memory { return &Memory{codes: map[string]string{}} }

func (m *Memory) Send(_ context.Context, phone, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[phone] = code
	return nil
}

// Last devuelve el último código enviado a ese celular (E.164).
func (m *Memory) Last(phone string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[phone]
}
