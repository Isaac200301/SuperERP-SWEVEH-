package domain

import (
	"testing"
	"time"
)

var now = time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)

// HU-01, criterio 2: después de 5 intentos fallidos se bloquea el celular 15 minutos.
func TestLock_FiveFailuresLockFifteenMinutes(t *testing.T) {
	var l Lock
	for i := 1; i <= 4; i++ {
		var remaining int
		l, remaining = l.RegisterFailure(now)
		if remaining != MaxFailures-i || l.Locked(now) {
			t.Fatalf("fallo %d: quedan %d, bloqueado=%v", i, remaining, l.Locked(now))
		}
	}
	l, remaining := l.RegisterFailure(now)
	if remaining != 0 || !l.Locked(now) {
		t.Fatalf("el quinto fallo debía bloquear: quedan %d, %+v", remaining, l)
	}
	if !l.Locked(now.Add(14*time.Minute + 59*time.Second)) {
		t.Fatal("debía seguir bloqueado antes de 15 minutos")
	}
	if l.Locked(now.Add(LockDuration)) {
		t.Fatal("a los 15 minutos se desbloquea")
	}
	// Tras vencer el bloqueo se empieza de cero.
	l, remaining = l.RegisterFailure(now.Add(LockDuration))
	if remaining != MaxFailures-1 || l.Locked(now.Add(LockDuration)) {
		t.Fatalf("después del bloqueo: quedan %d, %+v", remaining, l)
	}
}

// HU-01, criterio 1: el código vence a los 10 minutos.
func TestChallenge_ExpiresAtTenMinutes(t *testing.T) {
	c := Challenge{ExpiresAt: now.Add(CodeTTL)}
	if c.Expired(now.Add(9*time.Minute + 59*time.Second)) {
		t.Fatal("no debía vencer antes de 10 minutos")
	}
	if !c.Expired(now.Add(10 * time.Minute)) {
		t.Fatal("debía vencer a los 10 minutos")
	}
}

func TestValidCodeFormat(t *testing.T) {
	for _, ok := range []string{"000000", "123456"} {
		if !ValidCodeFormat(ok) {
			t.Errorf("%q es válido", ok)
		}
	}
	for _, bad := range []string{"", "12345", "1234567", "12a456", "١٢٣٤٥٦"} {
		if ValidCodeFormat(bad) {
			t.Errorf("%q no es válido", bad)
		}
	}
}

func TestErrCodeWrongMessage(t *testing.T) {
	if ErrCodeWrong(1).Message != "El código no coincide. Te queda 1 intento." {
		t.Fatal(ErrCodeWrong(1).Message)
	}
	if ErrCodeWrong(3).Message != "El código no coincide. Te quedan 3 intentos." {
		t.Fatal(ErrCodeWrong(3).Message)
	}
}

func TestHasPermission_DenyByDefault(t *testing.T) {
	if !HasPermission([]string{"dueno"}, PermOrderCreate) {
		t.Fatal("el dueño abre órdenes")
	}
	// HU-16: el mecánico abre órdenes (difiere de la matriz de docs/06, ver Q-30).
	if !HasPermission([]string{"mecanico"}, PermOrderCreate) {
		t.Fatal("el mecánico abre órdenes (HU-16)")
	}
	if HasPermission([]string{"mecanico"}, PermCustomerRead) {
		t.Fatal("el mecánico no ve todos los clientes")
	}
	if HasPermission([]string{"inventado"}, PermOrderRead) || HasPermission(nil, PermOrderRead) {
		t.Fatal("rol desconocido o vacío no tiene permisos")
	}
	if HasPermission([]string{"dueno"}, "permiso:inventado") {
		t.Fatal("permiso desconocido se niega")
	}
}

func TestParsePersonNameAndWorkshopName(t *testing.T) {
	n, err := ParsePersonName("  Hernán   Ruiz ")
	if err != nil || n != "Hernán Ruiz" {
		t.Fatalf("%q %v", n, err)
	}
	if _, err := ParsePersonName("   "); err == nil {
		t.Fatal("nombre vacío")
	}
	if DefaultWorkshopName("Hernán Ruiz") != "Taller de Hernán" || DefaultWorkshopName("") != "Mi taller" {
		t.Fatal(DefaultWorkshopName("Hernán Ruiz"))
	}
}
