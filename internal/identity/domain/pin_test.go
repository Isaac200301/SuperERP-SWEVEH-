package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// HU-104: 4 dígitos, sin los fáciles de adivinar.
func TestParsePIN(t *testing.T) {
	for _, ok := range []string{"2580", "0407", "9173"} {
		if _, err := ParsePIN(ok); err != nil {
			t.Fatalf("%s debía servir: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "123", "12345", "12a4", " 258", "２５８０"} {
		if _, err := ParsePIN(bad); !errors.Is(err, ErrPINFormat) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	for _, easy := range []string{"0000", "1111", "1234", "4321", "9876", "0123"} {
		if _, err := ParsePIN(easy); !errors.Is(err, ErrPINTrivial) {
			t.Fatalf("%s debía ser fácil: %v", easy, err)
		}
		if !ValidPINShape(easy) {
			t.Fatalf("%s tiene forma de PIN (al entrar no se juzga si es fácil)", easy)
		}
	}
	if ValidPINShape("12a4") || ValidPINShape("123") {
		t.Fatal("forma")
	}
}

// V-03: el bloqueo cae exactamente en el 5.º fallo, no en el 4.º, y dura lo que diga la política.
func TestLockPolicy(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	p := LockPolicy{MaxFailures: 5, Duration: 15 * time.Minute}
	var l Lock
	for i := 1; i <= 4; i++ {
		var left int
		l, left = l.RegisterFailureWith(now, p)
		if l.Locked(now) || left != 5-i {
			t.Fatalf("fallo %d: bloqueado=%v quedan=%d", i, l.Locked(now), left)
		}
	}
	l, left := l.RegisterFailureWith(now, p)
	if !l.Locked(now) || left != 0 || !l.Locked(now.Add(14*time.Minute)) || l.Locked(now.Add(15*time.Minute)) {
		t.Fatalf("5.º fallo: %+v %d", l, left)
	}
	if e := ErrLoginLocked(now.Add(10*time.Minute), l); !strings.Contains(e.Message, "5 minutos") {
		t.Fatal(e.Message)
	}
	if e := ErrLoginLocked(now.Add(14*time.Minute+30*time.Second), l); !strings.Contains(e.Message, "1 minuto.") {
		t.Fatal(e.Message)
	}
	// Vencido el bloqueo, se empieza de cero.
	l, left = l.RegisterFailureWith(now.Add(16*time.Minute), p)
	if l.Locked(now.Add(16*time.Minute)) || left != 4 {
		t.Fatalf("después del bloqueo: %+v %d", l, left)
	}
	if !strings.Contains(ErrLoginFailed(1).Message, "1 intento") || !strings.Contains(ErrLoginFailed(3).Message, "3 intentos") {
		t.Fatal("mensajes")
	}
	if _, err := ParseWorkshopName("  "); !errors.Is(err, ErrWorkshopName) {
		t.Fatal(err)
	}
	if n, _ := ParseWorkshopName(" Servitaller   El Pistón "); n != "Servitaller El Pistón" {
		t.Fatal(n)
	}
}
