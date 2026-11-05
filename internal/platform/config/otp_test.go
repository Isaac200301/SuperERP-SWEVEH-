package config

import (
	"strings"
	"testing"
)

// ADR-0034: en staging el código de entrada sale por el WhatsApp de prueba (OTP_SENDER=waha), que necesita WAHA
// configurado; "consola" y "memoria" siguen siendo solo de local. En producción WAHA no se permite.
func TestOTPSenderWAHA(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://x", "JWT_PRIVATE_KEY_PEM": "pem", "OTP_PEPPER": strings.Repeat("a", 32)}
	waha := map[string]string{"WAHA_URL": "http://waha", "WAHA_API_KEY": "k", "WAHA_WEBHOOK_SECRET": "s"}
	cases := []struct {
		name   string
		env    string
		extra  map[string]string
		errSub string
	}{
		{"staging con waha", "staging", merge(waha, map[string]string{"OTP_SENDER": "waha"}), ""},
		{"waha sin WAHA_URL", "staging", map[string]string{"OTP_SENDER": "waha"}, "OTP_SENDER=waha necesita WAHA_URL"},
		{"consola en staging", "staging", map[string]string{"OTP_SENDER": "consola"}, "solo se permite en local"},
		{"waha en producción", "production", merge(waha, map[string]string{"OTP_SENDER": "waha"}), "solo para la demo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"APP_ENV", "OTP_SENDER", "WAHA_URL", "WAHA_API_KEY", "WAHA_WEBHOOK_SECRET", "WHATSAPP_PROVIDER", "AI_PROVIDER"} {
				t.Setenv(k, "")
			}
			for k, v := range merge(base, tc.extra) {
				t.Setenv(k, v)
			}
			t.Setenv("APP_ENV", tc.env)
			t.Setenv("WHATSAPP_PROVIDER", "local")
			_, err := LoadAPI()
			got := ""
			if err != nil {
				got = err.Error()
			}
			if tc.errSub == "" && strings.Contains(got, "OTP_SENDER") {
				t.Fatalf("no esperaba error de OTP_SENDER: %v", err)
			}
			if tc.errSub != "" && !strings.Contains(got, tc.errSub) {
				t.Fatalf("error %q, esperaba %q", got, tc.errSub)
			}
		})
	}
}

func merge(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
