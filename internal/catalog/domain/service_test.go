package domain

import "testing"

func byName(list []Service) map[string]string {
	m := map[string]string{}
	for _, s := range list {
		m[s.Name] = s.Kind
	}
	return m
}

func TestPreload_BySpecialtyAndVehicleKind(t *testing.T) {
	moto := byName(Preload("moto", []string{"latoneria_pintura", "aire_acondicionado", "desconocida"}))
	if moto["Latonería"] != "moto" || moto["Cambio de kit de arrastre"] != "moto" {
		t.Errorf("taller de motos con latonería: %v", moto)
	}
	if _, ok := moto["Revisión de aire acondicionado"]; ok {
		t.Error("el aire acondicionado es de carro: no aplica a un taller de motos")
	}

	both := byName(Preload("ambos", []string{"llantas_alineacion", "sistema_electrico"}))
	if both["Cambio de aceite y filtro"] != "ambos" || both["Revisión eléctrica"] != "ambos" {
		t.Errorf("en ambos, lo común queda de ambos: %v", both)
	}
	if both["Alineación y balanceo"] != "carro" || both["Cambio de kit de arrastre"] != "moto" {
		t.Errorf("lo de un solo tipo conserva su tipo: %v", both)
	}
	if len(Preload("carro", nil)) != len(preloadCar) {
		t.Error("sin especialidades queda solo el mantenimiento básico")
	}
}

// HU-93: una fila leída de la foto se limpia antes de quedar por revisar.
func TestRowClean(t *testing.T) {
	neg, ok := int64(-5), true
	r, ok := Row{Name: "  Cambio   de aceite ", Kind: "bus", Price: &neg}.Clean()
	if !ok || r.Name != "Cambio de aceite" || r.Kind != "ambos" || r.Price != nil {
		t.Fatalf("limpieza: %+v %v", r, ok)
	}
	if _, ok := (Row{Name: "   "}).Clean(); ok {
		t.Fatal("una fila sin nombre no se guarda")
	}
}
