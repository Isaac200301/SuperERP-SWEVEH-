package domain

import (
	"errors"
	"fmt"
	"time"
)

// Fechas de las citas que agenda el asesor (ADR-0029). Todo en hora de Colombia.

// Bogota es la zona del taller. Sin la base de zonas del sistema se usa UTC-5 (Colombia no tiene horario de verano).
var Bogota = func() *time.Location {
	if l, err := time.LoadLocation("America/Bogota"); err == nil {
		return l
	}
	return time.FixedZone("COT", -5*3600)
}()

// MaxBookingDays es hasta cuántos días adelante se agenda por WhatsApp.
const MaxBookingDays = 30

var (
	ErrWhenFormat = errors.New("fecha u hora con formato inválido")
	ErrWhenPast   = errors.New("la fecha ya pasó")
	ErrWhenFar    = errors.New("la fecha está a más de 30 días")
)

// ParseWhen arma la cita con fecha "AAAA-MM-DD" y hora "HH:MM" en hora de Colombia y la valida contra now: no en el
// pasado ni a más de MaxBookingDays.
func ParseWhen(date, hour string, now time.Time) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02 15:04", date+" "+hour, Bogota)
	if err != nil {
		return time.Time{}, ErrWhenFormat
	}
	switch {
	case t.Before(now):
		return time.Time{}, ErrWhenPast
	case t.After(now.AddDate(0, 0, MaxBookingDays)):
		return time.Time{}, ErrWhenFar
	}
	return t, nil
}

var (
	weekdays = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}
	months   = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre",
		"noviembre", "diciembre"}
)

// Clock12 es la hora como se dice en Colombia: "9:00 a. m.", "2:30 p. m.".
func Clock12(t time.Time) string {
	h, suf := t.Hour(), "a. m."
	if h >= 12 {
		suf = "p. m."
	}
	if h > 12 {
		h -= 12
	}
	if h == 0 {
		h = 12
	}
	return fmt.Sprintf("%d:%02d %s", h, t.Minute(), suf)
}

// FormatWhen es la cita en palabras: "sábado 8 de noviembre, 9:00 a. m.".
func FormatWhen(t time.Time) string {
	t = t.In(Bogota)
	return fmt.Sprintf("%s %d de %s, %s", weekdays[t.Weekday()], t.Day(), months[t.Month()-1], Clock12(t))
}

// FormatToday es la fecha y hora de hoy para el prompt: "martes 4 de noviembre de 2026, 2:20 p. m.".
func FormatToday(t time.Time) string {
	t = t.In(Bogota)
	return fmt.Sprintf("%s %d de %s de %d, %s", weekdays[t.Weekday()], t.Day(), months[t.Month()-1], t.Year(), Clock12(t))
}
