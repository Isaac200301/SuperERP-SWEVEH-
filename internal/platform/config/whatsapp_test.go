package config

import (
	"strings"
	"testing"
)

// WHATSAPP_PROVIDER: "meta" si hay token, si no el simulador; meta necesita token y secreto; el simulador no va
// a producción (docs/07).
func TestWhatsAppProvider(t *testing.T) {
	cases := []struct {
		name      string
		env       Env
		vars      map[string]string
		provider  string
		errSubstr string
	}{
		{"sin nada: simulador", EnvLocal, nil, "local", ""},
		{"con token: meta", EnvStaging, map[string]string{"WHATSAPP_ACCESS_TOKEN": "t", "WHATSAPP_APP_SECRET": "s"}, "meta", ""},
		{"meta sin secreto", EnvStaging, map[string]string{"WHATSAPP_PROVIDER": "meta", "WHATSAPP_ACCESS_TOKEN": "t"}, "meta", "WHATSAPP_APP_SECRET"},
		{"simulador en staging", EnvStaging, map[string]string{"WHATSAPP_PROVIDER": "local"}, "local", ""},
		{"simulador en producción", EnvProduction, map[string]string{"WHATSAPP_PROVIDER": "local"}, "local", "producción"},
		{"desconocido", EnvLocal, map[string]string{"WHATSAPP_PROVIDER": "telegram"}, "telegram", "no es un canal conocido"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"WHATSAPP_PROVIDER", "WHATSAPP_ACCESS_TOKEN", "WHATSAPP_APP_SECRET"} {
				t.Setenv(k, tc.vars[k])
			}
			var errs []error
			wa, _ := engine(tc.env, &errs)
			if wa.Provider != tc.provider {
				t.Fatalf("proveedor %q, esperaba %q", wa.Provider, tc.provider)
			}
			got := ""
			if len(errs) > 0 {
				got = errs[0].Error()
			}
			if (tc.errSubstr == "") != (got == "") || !strings.Contains(got, tc.errSubstr) {
				t.Fatalf("error %q, esperaba que contuviera %q", got, tc.errSubstr)
			}
		})
	}
}
