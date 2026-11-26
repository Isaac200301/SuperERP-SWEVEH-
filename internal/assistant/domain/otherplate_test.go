package domain

import (
	"slices"
	"strings"
	"testing"
)

// Otro vehículo en el chat de una orden (prueba de Pablo, 26-nov): sus datos y fotos van a un borrador aparte; nunca
// a la orden del chat. La persona lo abre en otra conversación o lo descarta (las fotos vuelven).
func TestOtherOrderDraft(t *testing.T) {
	p := FloorProposal{OrderID: "o41", OrderCode: "OS-0041", Plate: &Field{Value: "XVN43F", Origin: FromWorkshop},
		Photos: []FloorPhoto{{Ref: "moto", Side: "frente", Origin: FromPhoto}, {Ref: "a", Side: "trasera", Origin: FromVideo}}}
	p.MovePhotosToOther([]FloorPhoto{{Ref: "a"}})
	if p.OtherOrder != nil || len(p.Photos) != 2 {
		t.Fatal("sin borrador no se mueve nada")
	}
	d := p.SetOther("TPR418", FromVideo)
	if d.Plate.Value != "TPR418" || !d.NewOrder {
		t.Fatalf("borrador: %+v", d)
	}
	solo := FloorProposal{OrderID: "o41"}
	if solo.SetOther("TPR418", FromSaid); solo.Pending() {
		t.Fatal("el borrador va aparte: no cuenta como pendiente de esta orden")
	}
	p.MovePhotosToOther([]FloorPhoto{{Ref: "a", Origin: FromVideo}, {Ref: "b", Origin: FromVideo}})
	if len(p.Photos) != 1 || p.Photos[0].Ref != "moto" || len(d.Photos) != 2 || d.Photos[0].Side != "trasera" || !p.InOther("b") || p.InOther("moto") {
		t.Fatalf("las fotos pasan al borrador con su lado: %+v / %+v", p.Photos, d.Photos)
	}
	if p.SetOther("TPR418", FromSaid) != d {
		t.Fatal("la misma placa retoma el borrador")
	}
	if got := strings.Join(d.OrderMissing(), "|"); got != "si es carro o moto|el celular y el nombre del cliente|qué trae" {
		t.Fatalf("falta: %q", got)
	}

	// Confirmar lo de la moto no borra la tarjeta del otro vehículo.
	if r := p.Reset("o41", "OS-0041", "recibido"); r.OtherOrder != d {
		t.Fatal("el borrador sigue después de confirmar")
	}

	// La ficha no edita el borrador: lo que mande la pantalla se ignora, y quitarlo devuelve las fotos.
	edited := HumanEdit(p, FloorProposal{Photos: p.Photos, OtherOrder: &FloorProposal{Plate: &Field{Value: "ZZZ999"}}})
	if edited.OtherOrder == nil || edited.OtherOrder.Plate.Value != "TPR418" {
		t.Fatalf("el borrador no se edita a mano: %+v", edited.OtherOrder)
	}
	dropped := HumanEdit(p, FloorProposal{Photos: p.Photos})
	if dropped.OtherOrder != nil || len(dropped.Photos) != 3 {
		t.Fatalf("No, sigo aquí: las fotos vuelven a esta orden: %+v", dropped.Photos)
	}

	// Otra placa nueva: el borrador anterior se descarta y sus fotos vuelven.
	p.SetOther("ABC123", FromSaid)
	if p.OtherOrder.Plate.Value != "ABC123" || len(p.OtherOrder.Photos) != 0 || len(p.Photos) != 3 {
		t.Fatalf("otra placa: %+v", p)
	}
}

func TestFindPlatesStrict(t *testing.T) {
	for text, want := range map[string][]string{
		"llegó a las 10:30 con el de la mañana": nil,
		"son 20 mil":                        nil,
		"también llegó el TPR-418 de Pedro": {"TPR418"},
		"la placa tpr418":                   {"TPR418"},
		"es la TPR 418":                     {"TPR418"},
		"orden OS-0041 de la XVN43F":        {"XVN43F"},
	} {
		if got := FindPlatesStrict(text); !slices.Equal(got, want) {
			t.Fatalf("%q: %v, quería %v", text, got, want)
		}
	}
}

func TestNearPlate(t *testing.T) {
	if !NearPlate("XVN43F", "XVN43F") || !NearPlate("XVN43E", "XVN43F") || NearPlate("TPR418", "XVN43F") || NearPlate("XVN43", "XVN43F") {
		t.Fatal("near plate")
	}
}
