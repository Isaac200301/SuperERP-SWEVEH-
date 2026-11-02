package shared

import (
	"errors"
	"testing"
)

func TestParseColombianMobile(t *testing.T) {
	ok := map[string]string{
		"3001234567":       "+573001234567",
		"300 123 4567":     "+573001234567",
		"300-123-4567":     "+573001234567",
		"+57 300 123 4567": "+573001234567",
		"573001234567":     "+573001234567",
		"(300) 123.45.67":  "+573001234567",
	}
	for in, want := range ok {
		got, err := ParseColombianMobile(in)
		if err != nil || got != want {
			t.Errorf("ParseColombianMobile(%q) = %q, %v; quiero %q", in, got, err, want)
		}
	}
	// HU-09, criterio 2: menos o más de 10 dígitos se rechaza con el formato.
	bad := []string{"", "300123456", "30012345678", "6012345678", "abc1234567", "+1 300 123 4567", "3001234567x"}
	for _, in := range bad {
		if _, err := ParseColombianMobile(in); !errors.Is(err, ErrPhoneInvalid) {
			t.Errorf("ParseColombianMobile(%q) debía fallar con celular_invalido, dio %v", in, err)
		}
	}
}

func TestErrorIsComparesByCode(t *testing.T) {
	withField := ErrPhoneInvalid.WithField("celular")
	if !errors.Is(withField, ErrPhoneInvalid) {
		t.Fatal("una copia con campo debe ser el mismo error")
	}
	if errors.Is(withField, ErrVersionConflict) {
		t.Fatal("códigos distintos no son el mismo error")
	}
}
