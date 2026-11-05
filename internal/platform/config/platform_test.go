package config

import (
	"strings"
	"testing"
)

// HU-74 (ADR-0030): PLATFORM_ADMINS son celulares colombianos y quedan en E.164; uno mal escrito no deja arrancar.
// Mensajes incluidos y pesos por dólar tienen valor provisional (D-04, Q-24) y deben ser positivos.
func TestPlatformAdmins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("PLATFORM_ADMINS", "310 555 0640, +573105550641")
	t.Setenv("PLAN_MENSAJES_INCLUIDOS", "")
	t.Setenv("TRM_COP_POR_USD", "")
	c, err := LoadAPI()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Platform.Admins) != 2 || c.Platform.Admins[0] != "+573105550640" || c.Platform.IncludedMessages != 1000 || c.Platform.COPPerUSD != 4000 {
		t.Fatalf("plataforma: %+v", c.Platform)
	}
	t.Setenv("PLATFORM_ADMINS", "123")
	if _, err := LoadAPI(); err == nil || !strings.Contains(err.Error(), "PLATFORM_ADMINS") {
		t.Fatalf("celular inválido: %v", err)
	}
	t.Setenv("PLATFORM_ADMINS", "")
	t.Setenv("PLAN_MENSAJES_INCLUIDOS", "0")
	if _, err := LoadAPI(); err == nil || !strings.Contains(err.Error(), "PLAN_MENSAJES_INCLUIDOS") {
		t.Fatalf("incluidos en cero: %v", err)
	}
}
