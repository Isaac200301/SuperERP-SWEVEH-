package domain

import (
	"strconv"
	"testing"
	"time"
)

var consent = []Button{{ID: BtnConsentYes, Title: "Acepto"}, {ID: BtnConsentNo, Title: "No acepto"}}

func TestMenu(t *testing.T) {
	got := Menu("¿Nos autorizas?\n", consent)
	want := "¿Nos autorizas?\n\nResponde con el número:\n1. Acepto\n2. No acepto"
	if got != want {
		t.Fatalf("%q", got)
	}
	if Menu("hola", nil) != "hola" {
		t.Fatal("sin opciones el texto queda igual")
	}
}

func TestMatchReply(t *testing.T) {
	quote := []Button{{ID: "quote:aprobar:1", Title: "✅ Aprobar"}, {ID: "quote:dudas:1", Title: "Tengo dudas"}}
	cases := []struct {
		reply string
		b     []Button
		want  string
	}{
		{"1", consent, BtnConsentYes},
		{" 2. ", consent, BtnConsentNo},
		{"ACEPTO!!", consent, BtnConsentYes},
		{"no acepto", consent, BtnConsentNo},
		{"aprobar", quote, "quote:aprobar:1"},
		{"tengo dudas 🤔", quote, "quote:dudas:1"},
		{"3", consent, ""},
		{"10", consent, ""},
		{"0", consent, ""},
		{"acepto pero tengo una pregunta", consent, ""},
		{"¿cuánto vale?", consent, ""},
		{"", consent, ""},
		{"1", nil, ""},
	}
	for _, c := range cases {
		got, ok := MatchReply(c.reply, c.b)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("MatchReply(%q) = %q, %v; quiero %q", c.reply, got, ok, c.want)
		}
	}
}

func TestRules(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	in, out := now.Add(-23*time.Hour), now.Add(-25*time.Hour)
	if !InWindow(&in, now) || InWindow(&out, now) || InWindow(nil, now) {
		t.Fatal("ventana de 24 h")
	}
	list := make([]Button, MaxListRows)
	for i := range list {
		list[i] = Button{ID: "v" + strconv.Itoa(i), Title: "Opción de 24 caracteres."}
	}
	for name, c := range map[string]struct {
		b  []Button
		ok bool
	}{
		"dos botones":           {consent, true},
		"título largo en botón": {[]Button{{ID: "a", Title: "Un título de 21 letras"}}, false},
		"lista de 10":           {list, true},
		"lista de 11":           {append(list, Button{ID: "x", Title: "x"}), false},
		"sin id":                {[]Button{{Title: "Acepto"}}, false},
	} {
		if err := ValidateButtons(c.b); (err == nil) != c.ok {
			t.Errorf("%s: %v", name, err)
		}
	}
	if IsList(consent) || !IsList(list) {
		t.Fatal("IsList")
	}
}
