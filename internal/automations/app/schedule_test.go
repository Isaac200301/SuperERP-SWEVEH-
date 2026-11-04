package app

import (
	"testing"
	"time"
)

// HU-43: el aviso de "listo" dice el horario de hoy si todavía se puede pasar; si ya cerró o hoy no abre, el del
// siguiente día con horario. Horas en Colombia (UTC-5).
func TestScheduleText_NextOpenDay(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	week := map[string]*[2]string{
		"lun": {"07:30", "17:30"}, "mar": {"07:30", "17:30"}, "mie": {"07:30", "17:30"}, "jue": {"07:30", "17:30"},
		"vie": {"07:30", "17:30"}, "sab": {"08:00", "12:00"},
	}
	w := Workshop{Schedule: week}
	col := time.FixedZone("COT", -5*3600)
	at := func(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, col) } // 2-nov-2026 es lunes
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"lunes antes de abrir", at(2, 6, 0), "hoy de 7:30 a. m. a 5:30 p. m."},
		{"lunes abierto", at(2, 15, 0), "hoy de 7:30 a. m. a 5:30 p. m."},
		{"lunes ya cerró", at(2, 18, 0), "mañana de 7:30 a. m. a 5:30 p. m."},
		{"sábado al mediodía cerró", at(7, 12, 0), "el lunes de 7:30 a. m. a 5:30 p. m."},
		{"domingo no abre", at(8, 10, 0), "mañana de 7:30 a. m. a 5:30 p. m."},
		{"viernes en la noche", at(6, 20, 0), "mañana de 8:00 a. m. a 12:00 p. m."},
	}
	for _, tc := range cases {
		if got := s.ScheduleText(w, tc.at.UTC()); got != tc.want {
			t.Errorf("%s: %q, esperaba %q", tc.name, got, tc.want)
		}
	}
	if got := s.ScheduleText(Workshop{}, at(2, 10, 0)); got != "en nuestro horario de atención" {
		t.Errorf("sin horario: %q", got)
	}
}
