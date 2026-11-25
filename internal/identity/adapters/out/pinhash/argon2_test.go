package pinhash

import (
	"strings"
	"testing"
)

// Parámetros chicos para que la prueba sea rápida; el formato y la comparación son los mismos.
var fast = Params{MemoryKiB: 64, Time: 1, Threads: 1}

func TestHashAndVerify(t *testing.T) {
	h, err := New([]byte("pepper-de-pruebas-de-32-caracteres!!"), fast)
	if err != nil {
		t.Fatal(err)
	}
	a, err := h.Hash("2580")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, "$argon2id$v=19$m=64,t=1,p=1$") || strings.Contains(a, "2580") {
		t.Fatalf("formato: %s", a)
	}
	b, _ := h.Hash("2580")
	if a == b {
		t.Fatal("dos hash del mismo PIN deben tener sal distinta")
	}
	if !h.Verify("2580", a) || h.Verify("2581", a) {
		t.Fatal("verificar")
	}
	// Sin hash (celular sin cuenta) o dañado: false, sin fallar.
	for _, enc := range []string{"", "$argon2id$v=19$m=x$", "$bcrypt$...", "$argon2id$v=19$m=64,t=1,p=1$c2Fs$"} {
		if h.Verify("2580", enc) {
			t.Fatalf("debía fallar con %q", enc)
		}
	}
	// Otro pepper no valida el mismo hash: el pepper no está en la base.
	other, _ := New([]byte("otro-pepper-de-pruebas-de-32-letras"), fast)
	if other.Verify("2580", a) {
		t.Fatal("el pepper debe importar")
	}
	if _, err := New(nil, fast); err == nil {
		t.Fatal("sin pepper debe fallar")
	}
}
