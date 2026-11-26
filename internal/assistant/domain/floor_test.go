package domain

import (
	"errors"
	"testing"
)

func i64(v int64) *int64 { return &v }

func TestSaidAmounts(t *testing.T) {
	got := SaidAmounts("el torno me costó $120.000 y la mano de obra 80 mil", "kilometraje 45000", "pastillas a 95,000")
	for _, want := range []int64{120000, 80000, 45000, 95000} {
		if !got[want] {
			t.Errorf("faltó %d en %v", want, got)
		}
	}
	if got[100000] {
		t.Error("100.000 no lo dijo nadie")
	}
}

// RF-17: un precio solo sale de la lista del taller o de lo que dijo la persona. Lo demás queda sin precio.
func TestPriceLine(t *testing.T) {
	list := []ListService{{ID: "s1", Name: "Cambio de aceite", Price: i64(45000)}, {ID: "s2", Name: "Alineación"}}
	said := SaidAmounts("la alineación cóbrala a 60 mil")
	cases := []struct {
		name     string
		in       FloorLine
		price    *int64
		origin   string
		dropped  bool
		serviceI string
	}{
		{"precio de la lista", FloorLine{Kind: "servicio", Name: "cambio de ACEITE", Price: i64(45000)}, i64(45000), FromList, false, "s1"},
		{"la IA cambia el de la lista: manda la lista", FloorLine{Kind: "servicio", Name: "Cambio de aceite", Price: i64(99000)}, i64(45000), FromList, true, "s1"},
		{"sin precio en la lista, la persona lo dijo", FloorLine{Kind: "servicio", Name: "Alineación", Price: i64(60000)}, i64(60000), FromSaid, false, "s2"},
		{"sin precio en la lista, inventado", FloorLine{Kind: "servicio", Name: "Alineación", Price: i64(70000)}, nil, "", true, "s2"},
		{"partida inventada", FloorLine{Kind: "partida", Name: "Pintura", Price: i64(100000)}, nil, "", true, ""},
		{"repuesto dicho", FloorLine{Kind: "repuesto", Name: "Filtro", Price: i64(60000)}, i64(60000), FromSaid, false, ""},
		{"sin precio", FloorLine{Kind: "repuesto", Name: "Filtro"}, nil, "", false, ""},
	}
	for _, c := range cases {
		out, dropped := PriceLine(c.in, list, said)
		if dropped != c.dropped || out.PriceOrigin != c.origin || out.ServiceID != c.serviceI || !samePrice(out.Price, c.price) {
			t.Errorf("%s: %+v dropped=%v", c.name, out, dropped)
		}
	}
}

func TestFloorTextOK(t *testing.T) {
	allowed := map[int64]bool{45000: true}
	if !FloorTextOK("El cambio de aceite queda en $45.000.", allowed) {
		t.Error("el de la lista debe pasar")
	}
	if FloorTextOK("Te cobro $100.000 por todo.", allowed) {
		t.Error("una cifra que nadie dijo no pasa")
	}
	for _, s := range []string{"Las pastillas quedan en 95 mil.", "Las pastillas quedan en 95.000."} {
		if FloorTextOK(s, allowed) {
			t.Errorf("sin $ también se revisa: %q", s)
		}
	}
	if !FloorTextOK("Anoté 87.000 kilómetros, 45000 km y 4 fotos; el aceite en 45 mil.", allowed) {
		t.Error("el kilometraje y los números sueltos no son plata")
	}
}

func TestFindPlates(t *testing.T) {
	got := FindPlates("llegaron el abc123 y la moto XYZ98D, y otra vez ABC-123")
	if len(got) != 2 || got[0] != "ABC123" || got[1] != "XYZ98D" {
		t.Fatalf("placas: %v", got)
	}
}

func TestProgress(t *testing.T) {
	p := FloorProposal{Plate: &Field{Value: "ABC123"}, NewOrder: true, Kind: "carro", Symptom: &Field{Value: "frenos"},
		Km: &Field{Value: "45000"}, Fuel: &Field{Value: "1/2"},
		Photos: []FloorPhoto{{Ref: "a", Side: "frente"}, {Ref: "b", Side: "izquierdo"}, {Ref: "c", Side: "derecho"}}}
	pr := p.Progress(Saved{})
	if pr.Have != 7 || pr.Total != 8 || len(pr.Missing) != 1 || pr.Missing[0] != "la parte trasera" {
		t.Fatalf("progreso: %+v", pr)
	}
	// Moto: 5 lados con el tablero; un vehículo nuevo pide el cliente.
	p.Kind, p.VehicleNew = "moto", true
	pr = p.Progress(Saved{Sides: []string{"trasera"}})
	if pr.Total != 10 || pr.Have != 8 {
		t.Fatalf("moto: %+v", pr)
	}
	if (FloorProposal{OrderID: "x", OrderState: "en_reparacion"}).Progress(Saved{}).Total != 0 {
		t.Fatal("fuera de la recepción no cuenta datos")
	}
	// Una orden "Por recibir" con la recepción ya guardada: lo guardado cuenta como hecho.
	saved := Saved{Sides: CarSides, Symptom: true, Km: true, Fuel: true}
	if pr := (FloorProposal{OrderID: "x", OrderState: "por_recibir", Plate: &Field{Value: "ABC123"}}).Progress(saved); pr.Have != pr.Total || pr.Total != 8 {
		t.Fatalf("lo guardado cuenta: %+v", pr)
	}
}

func TestValidate(t *testing.T) {
	ok := FloorProposal{OrderID: "o1", Quote: []FloorLine{{Name: "Frenos", Qty: 1, Price: i64(1)}}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		p    FloorProposal
		want error
	}{
		{FloorProposal{}, ErrFloorEmpty},
		{FloorProposal{Symptom: &Field{Value: "x"}}, ErrFloorNeedsOrder},
		{FloorProposal{NewOrder: true, Symptom: &Field{Value: "x"}}, ErrFloorPlate},
		{FloorProposal{NewOrder: true, VehicleNew: true, Plate: &Field{Value: "ABC123"}}, ErrFloorKind},
		{FloorProposal{NewOrder: true, VehicleNew: true, Kind: "carro", Plate: &Field{Value: "ABC123"}}, ErrFloorCustomer},
		{FloorProposal{OrderID: "o1", Quote: []FloorLine{{Name: "Frenos", Qty: 1}}}, ErrFloorPrice},
		{FloorProposal{OrderID: "o1", Quote: []FloorLine{{Name: "", Qty: 1, Price: i64(1)}}}, ErrFloorItem},
		{FloorProposal{OrderID: "o1", Expenses: []FloorExpense{{Description: "torno"}}}, ErrFloorExpense},
		{FloorProposal{OrderID: "o1", Fuel: &Field{Value: "medio"}}, ErrFloorFuel},
		{FloorProposal{OrderID: "o1", Km: &Field{Value: "45.000"}}, ErrFloorKm},
		{FloorProposal{OrderID: "o1", Findings: []FloorFinding{{System: "frenos"}}}, ErrFloorFinding},
	}
	for i, c := range cases {
		if err := c.p.Validate(); !errors.Is(err, c.want) {
			t.Errorf("caso %d: %v, quería %v", i, err, c.want)
		}
	}
}

// Lo que la persona cambia en la ficha queda con origen "pusiste"; lo que no cambia conserva el suyo y lo que
// identifica la orden no se edita desde la pantalla.
func TestHumanEdit(t *testing.T) {
	prev := FloorProposal{OrderID: "o1", OrderCode: "OS-0001", Plate: &Field{Value: "ABC123", Origin: FromWorkshop}, Kind: "carro",
		Quote: []FloorLine{
			{ID: "l1", Kind: "servicio", Name: "Aceite", Qty: 1, Price: i64(45000), PriceOrigin: FromList, ServiceID: "s1"},
			{ID: "l2", Kind: "repuesto", Name: "Filtro", Qty: 1},
		},
		Photos: []FloorPhoto{{Ref: "f1", Side: "frente", Origin: FromPhoto}},
		Km:     &Field{Value: "45000", Origin: FromSaid}}
	next := prev
	next.OrderID, next.OrderCode = "otra", "OS-9999"
	next.Plate = &Field{Value: "ZZZ999", Origin: FromList}
	next.Quote = []FloorLine{
		{ID: "l1", Kind: "servicio", Name: "Aceite", Qty: 1, Price: i64(45000), PriceOrigin: FromYou, ServiceID: "inventado"},
		{ID: "l2", Kind: "repuesto", Name: "Filtro", Qty: 1, Price: i64(32000), PriceOrigin: FromList},
		{ID: "", Kind: "partida", Name: "Pintura", Qty: 1, Price: i64(200000), PriceOrigin: FromList},
	}
	next.Photos = []FloorPhoto{{Ref: "f1", Side: "trasera", Origin: FromPhoto}, {Ref: "ajena", Origin: FromPhoto}}
	next.Km = &Field{Value: "46000", Origin: FromSaid}
	out := HumanEdit(prev, next)
	switch {
	case out.OrderID != "o1" || out.OrderCode != "OS-0001" || out.Plate.Value != "ABC123":
		t.Fatalf("la orden no se edita: %+v", out)
	case out.Quote[0].PriceOrigin != FromList || out.Quote[0].ServiceID != "s1":
		t.Fatalf("sin cambio conserva su origen: %+v", out.Quote[0])
	case out.Quote[1].PriceOrigin != FromYou || out.Quote[2].PriceOrigin != FromYou || out.Quote[2].ID == "":
		t.Fatalf("lo nuevo es de la persona: %+v", out.Quote)
	case len(out.Photos) != 1 || out.Photos[0].Side != "trasera" || out.Photos[0].Origin != FromYou:
		t.Fatalf("fotos: %+v", out.Photos)
	case out.Km.Origin != FromYou || out.Km.Value != "46000":
		t.Fatalf("km: %+v", out.Km)
	}
}

func TestSniffImage(t *testing.T) {
	if SniffImage([]byte{0xFF, 0xD8, 0xFF, 0xE0}) != "image/jpeg" || SniffImage([]byte("\x89PNG\r\n\x1a\nxx")) != "image/png" ||
		SniffImage([]byte("RIFF1234WEBPVP8 ")) != "image/webp" || SniffImage([]byte("<svg")) != "" {
		t.Fatal("tipos de foto")
	}
}

func TestPendingAndReset(t *testing.T) {
	p := FloorProposal{Plate: &Field{Value: "ABC123"}, Kind: "carro", Quote: []FloorLine{{Name: "x"}}}
	if !p.Pending() {
		t.Fatal("con cotización hay algo pendiente")
	}
	r := p.Reset("o1", "OS-0001", "recibido")
	if r.Pending() || r.OrderID != "o1" || r.Plate.Value != "ABC123" {
		t.Fatalf("reset: %+v", r)
	}
}
