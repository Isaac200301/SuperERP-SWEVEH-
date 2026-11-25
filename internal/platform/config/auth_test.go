package config

import (
	"strings"
	"testing"
	"time"
)

// HU-104 (ADR-0037): por defecto se entra con PIN; fuera de local el pepper es obligatorio y OTP_SENDER ya no se pide.
func TestAuthMode(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://x", "JWT_PRIVATE_KEY_PEM": "pem"}
	cases := []struct {
		name      string
		env       string
		vars      map[string]string
		errSubstr string
	}{
		{"por defecto PIN en local", "local", nil, ""},
		{"PIN en staging sin pepper", "staging", nil, "PIN_PEPPER"},
		{"PIN en staging con OTP_PEPPER, sin OTP_SENDER", "staging", map[string]string{"OTP_PEPPER": strings.Repeat("a", 32), "OTP_SENDER": "consola"}, ""},
		{"PIN_PEPPER propio", "staging", map[string]string{"OTP_PEPPER": strings.Repeat("a", 32), "PIN_PEPPER": "corto"}, "PIN_PEPPER"},
		{"modo desconocido", "local", map[string]string{"AUTH_MODE": "sms"}, "AUTH_MODE"},
		{"intentos inválidos", "local", map[string]string{"PIN_MAX_INTENTOS": "0"}, "PIN_MAX_INTENTOS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleanEngineEnv(t)
			for _, k := range []string{"AUTH_MODE", "OTP_PEPPER", "PIN_PEPPER", "OTP_SENDER", "PIN_MAX_INTENTOS", "PIN_BLOQUEO", "SESSION_REFRESH_TTL",
				"PIN_ARGON_MEMORY_KIB", "PIN_ARGON_TIME", "PIN_ARGON_THREADS", "WAHA_URL", "PLATFORM_ADMINS"} {
				t.Setenv(k, "")
			}
			for k, v := range base {
				t.Setenv(k, v)
			}
			for k, v := range tc.vars {
				t.Setenv(k, v)
			}
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("WHATSAPP_PROVIDER", "local")
			c, err := LoadAPI()
			got := ""
			if err != nil {
				got = err.Error()
			}
			if (tc.errSubstr == "") != (got == "") || !strings.Contains(got, tc.errSubstr) {
				t.Fatalf("error %q, esperaba %q", got, tc.errSubstr)
			}
			if tc.errSubstr == "" {
				a := c.Auth
				if !a.PIN() || a.MaxFailures != 5 || a.Lock != 15*time.Minute || a.RefreshTTL != 720*time.Hour || a.ArgonMemoryKiB != 19456 {
					t.Fatalf("valores por defecto: %+v", a)
				}
			}
		})
	}
}

// cmd/pin-reset y cmd/demo: el pepper local solo en local; fuera exige el mismo pepper del api (≥ 32).
func TestPINPepperFromEnv(t *testing.T) {
	for _, tc := range []struct{ env, pin, otp, want, err string }{
		{"local", "", "", LocalPepper, ""},
		{"", "", "", LocalPepper, ""},
		{"local", "", strings.Repeat("o", 32), strings.Repeat("o", 32), ""},
		{"staging", strings.Repeat("p", 32), strings.Repeat("o", 32), strings.Repeat("p", 32), ""},
		{"staging", "", "", "", "32 caracteres"},
		{"staging", "corto", "", "", "32 caracteres"},
	} {
		t.Setenv("APP_ENV", tc.env)
		t.Setenv("PIN_PEPPER", tc.pin)
		t.Setenv("OTP_PEPPER", tc.otp)
		got, err := PINPepperFromEnv()
		if got != tc.want || (tc.err == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), tc.err)) {
			t.Fatalf("%+v: %q %v", tc, got, err)
		}
	}
}
