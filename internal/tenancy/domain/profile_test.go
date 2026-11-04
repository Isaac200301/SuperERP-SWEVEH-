package domain

import (
	"slices"
	"testing"
)

func TestProfile_CitySpecialtiesAndLegacyKeys(t *testing.T) {
	p := Profile{Attends: "ambos", Size: "solo", City: "  santa rosa   DE osos ",
		Specialties: []string{"electrico", "sistema_electrico", "diagnostico_escaner", "alto_voltaje", "inventada"}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if p.City != "Santa Rosa de Osos" {
		t.Errorf("ciudad: %q", p.City)
	}
	if !slices.Equal(p.Specialties, []string{"sistema_electrico", "diagnostico_electronico", "alto_voltaje"}) {
		t.Errorf("especialidades: %v", p.Specialties)
	}
}
