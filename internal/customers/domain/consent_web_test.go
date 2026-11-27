package domain

import (
	"os"
	"strings"
	"testing"
)

// La pantalla muestra el mismo texto que se guarda como prueba de la autorización (HU-86, R-10): si uno cambia sin el
// otro, el cliente habría autorizado un texto distinto al registrado.
func TestCounterConsentTextMatchesWeb(t *testing.T) {
	b, err := os.ReadFile("../../../web/src/shared/legal/consent.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "'"+CounterConsentText+"'") {
		t.Fatalf("web/src/shared/legal/consent.ts no tiene el texto de CounterConsentText: %q", CounterConsentText)
	}
}
