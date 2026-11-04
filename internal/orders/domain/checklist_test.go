package domain

import (
	"errors"
	"testing"
)

func TestChecklist_MissingFollowsWorkshopRules(t *testing.T) {
	km, fuel := 1000, "1/2"
	car := Checklist{Kind: "carro", Km: &km, Fuel: &fuel}
	if m := car.Missing([]string{"frente"}, DefaultReceptionRules); len(m) != 1 || m[0] != "fotos: trasera, izquierdo, derecho" {
		t.Errorf("por defecto el carro pide los 4 lados: %v", m)
	}
	if m := car.Missing([]string{"frente", "trasera"}, ReceptionRules{MinPhotos: 2}); len(m) != 0 {
		t.Errorf("con mínimo 2 alcanza con 2 lados: %v", m)
	}
	if m := car.Missing(nil, ReceptionRules{MinPhotos: 0}); len(m) != 0 {
		t.Errorf("con mínimo 0 no se piden fotos: %v", m)
	}
	sinKm := Checklist{Kind: "moto", Fuel: &fuel}
	sides := []string{"frente", "trasera", "izquierdo", "derecho"}
	if m := sinKm.Missing(sides, DefaultReceptionRules); len(m) != 1 || m[0] != "kilometraje" {
		t.Errorf("kilometraje obligatorio por defecto: %v", m)
	}
	if m := sinKm.Missing(sides, ReceptionRules{MinPhotos: 4}); len(m) != 0 {
		t.Errorf("si el taller no exige kilometraje no falta: %v", m)
	}
	if m := sinKm.Missing([]string{"frente"}, ReceptionRules{MinPhotos: 3}); len(m) != 1 || m[0] != "fotos (mínimo 3 de: trasera, izquierdo, derecho, tablero)" {
		t.Errorf("moto con mínimo 3: %v", m)
	}
}

func TestChecklist_ItemStates(t *testing.T) {
	c := Checklist{Kind: "carro", Items: []ItemCheck{{Key: "luces", Name: "Luces", State: "mal", Note: "  stop derecho fundido "}, {Key: "radio", Name: "Radio"}}}
	if err := c.Validate(); err != nil || c.Items[0].Note != "stop derecho fundido" {
		t.Fatalf("%+v %v", c.Items, err)
	}
	c.Items[1].State = "regular"
	if err := c.Validate(); !errors.Is(err, ErrItemState) {
		t.Errorf("estado inválido: %v", err)
	}
}
