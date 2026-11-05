package analytics

import (
	"testing"
	"time"
)

// HU-70: los días hábiles no cuentan sábado ni domingo, con fracción.
func TestBusinessDays(t *testing.T) {
	loc := time.UTC
	fri := time.Date(2026, 11, 6, 12, 0, 0, 0, loc) // viernes al mediodía
	for _, tc := range []struct {
		name string
		a, b time.Time
		want float64
	}{
		{"mismo día", fri, fri.Add(6 * time.Hour), 0.25},
		{"viernes a lunes al mediodía", fri, fri.AddDate(0, 0, 3), 1},
		{"una semana", fri, fri.AddDate(0, 0, 7), 5},
		{"al revés", fri, fri.Add(-time.Hour), 0},
	} {
		if got := BusinessDays(tc.a, tc.b); got < tc.want-1e-9 || got > tc.want+1e-9 {
			t.Errorf("%s: %v; quiero %v", tc.name, got, tc.want)
		}
	}
	if m := median([]float64{12, 1, 30}); m == nil || *m != 12 {
		t.Errorf("mediana impar: %v", m)
	}
	if m := median([]float64{10, 20}); m == nil || *m != 15 {
		t.Errorf("mediana par: %v", m)
	}
	if median(nil) != nil || money(1234567) != "$1.234.567" {
		t.Error("mediana vacía o pesos")
	}
}
