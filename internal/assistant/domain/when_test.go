package domain

import "testing"

// HU-30: los números de ítems como los escribe un cliente.
func TestParsePicks(t *testing.T) {
	for text, want := range map[string][]int{"1 y 3": {1, 3}, "3,1": {1, 3}, "2": {2}, " 1 - 2 - 2 ": {1, 2}, "1. 2.": {1, 2}} {
		got, ok := ParsePicks(text)
		if !ok || len(got) != len(want) {
			t.Errorf("%q: %v %v", text, got, ok)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%q: %v", text, got)
			}
		}
	}
	for _, text := range []string{"", "sí", "el 1 por favor", "0", "1 y dos", "100"} {
		if got, ok := ParsePicks(text); ok {
			t.Errorf("%q no son números de ítems: %v", text, got)
		}
	}
}
