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

// AI_PROVIDER: reglas por defecto; openrouter necesita clave y modelo y no va a producción hasta AQ-09 (ADR-0026).
func TestAIProvider(t *testing.T) {
	cases := []struct {
		name      string
		env       Env
		vars      map[string]string
		errSubstr string
	}{
		{"sin nada: reglas", EnvProduction, nil, ""},
		{"openrouter en local", EnvLocal, map[string]string{"AI_PROVIDER": "openrouter", "AI_API_KEY": "k", "AI_MODEL": "m"}, ""},
		{"openrouter sin modelo", EnvLocal, map[string]string{"AI_PROVIDER": "openrouter", "AI_API_KEY": "k"}, "AI_MODEL"},
		{"openrouter en producción", EnvProduction, map[string]string{"AI_PROVIDER": "openrouter", "AI_API_KEY": "k", "AI_MODEL": "m"}, "AQ-09"},
		{"desconocido", EnvLocal, map[string]string{"AI_PROVIDER": "gpt"}, "no es un proveedor conocido"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"AI_PROVIDER", "AI_API_KEY", "AI_MODEL", "WHATSAPP_PROVIDER", "WHATSAPP_ACCESS_TOKEN"} {
				t.Setenv(k, tc.vars[k])
			}
			if tc.env == EnvProduction {
				t.Setenv("WHATSAPP_PROVIDER", "meta")
				t.Setenv("WHATSAPP_ACCESS_TOKEN", "t")
				t.Setenv("WHATSAPP_APP_SECRET", "s")
			}
			var errs []error
			_, ai := engine(tc.env, &errs)
			got := ""
			for _, e := range errs {
				got += e.Error()
			}
			if (tc.errSubstr == "") != (got == "") || !strings.Contains(got, tc.errSubstr) {
				t.Fatalf("error %q, esperaba que contuviera %q (proveedor %q)", got, tc.errSubstr, ai.Provider)
			}
		})
	}
}
