package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"sweveh/internal/shared"
)

var now = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

func ptr(s string) *string { return &s }

// HU-09, criterio 3: un cliente nuevo queda con autorización "pendiente".
func TestNewCustomer_StartsWithPendingConsent(t *testing.T) {
	c, err := NewCustomer(shared.NewID(), shared.NewID(), "300 123 4567", "  Laura   Gómez ", ptr(" Laura@Correo.COM "), now)
	if err != nil {
		t.Fatal(err)
	}
	if c.Phone != "+573001234567" || c.Name != "Laura Gómez" || *c.Email != "laura@correo.com" || c.Consent != ConsentPending {
		t.Fatalf("%+v", c)
	}
}

func TestNewCustomer_Validation(t *testing.T) {
	cases := []struct {
		phone, name string
		email       *string
		err         *shared.Error
	}{
		{"30012345", "Laura", nil, shared.ErrPhoneInvalid}, // HU-09, criterio 2
		{"3001234567", "", nil, ErrNameRequired},
		{"3001234567", strings.Repeat("a", 121), nil, ErrNameTooLong},
		{"3001234567", "Laura", ptr("no-es-correo"), ErrEmailInvalid},
	}
	for _, c := range cases {
		if _, err := NewCustomer(shared.NewID(), shared.NewID(), c.phone, c.name, c.email, now); !errors.Is(err, c.err) {
			t.Errorf("%+v: quería %v, dio %v", c, c.err, err)
		}
	}
	// Correo vacío = sin correo (es opcional).
	c, err := NewCustomer(shared.NewID(), shared.NewID(), "3001234567", "Laura", ptr("  "), now)
	if err != nil || c.Email != nil {
		t.Fatalf("correo vacío: %+v %v", c.Email, err)
	}
}

// ADR-0029: solo un nombre de relleno se reemplaza con el que el cliente le da al asesor.
func TestIsPlaceholderName(t *testing.T) {
	for name, want := range map[string]bool{"": true, "Cliente WhatsApp": true, "cliente de prueba": true, "J": true, "+573001234567": true,
		"Laura Gómez": false, "Carlos": false} {
		if got := IsPlaceholderName(name); got != want {
			t.Errorf("IsPlaceholderName(%q) = %v", name, got)
		}
	}
	if _, err := CleanName("   "); !errors.Is(err, ErrNameRequired) {
		t.Errorf("vacío: %v", err)
	}
	if n, err := CleanName("  Carlos   Ruiz "); err != nil || n != "Carlos Ruiz" {
		t.Errorf("CleanName = %q %v", n, err)
	}
}
