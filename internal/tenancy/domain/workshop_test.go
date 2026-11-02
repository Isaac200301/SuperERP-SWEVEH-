package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"sweveh/internal/shared"
)

func TestNewWorkshop(t *testing.T) {
	now := time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)
	w, err := NewWorkshop(shared.NewID(), "  Servitaller   El Pistón ", now)
	if err != nil || w.Name != "Servitaller El Pistón" || w.Version != 1 {
		t.Fatalf("%+v %v", w, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("a", 121)} {
		if _, err := NewWorkshop(shared.NewID(), bad, now); !errors.Is(err, ErrWorkshopName) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}
