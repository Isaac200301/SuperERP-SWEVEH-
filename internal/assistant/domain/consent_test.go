package domain

import (
	"errors"
	"slices"
	"testing"
)

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

// R11 (HU-86): un cliente nuevo no se confirma sin su respuesta a la autorización de datos; "no" también es respuesta.
func TestValidateNeedsCounterConsent(t *testing.T) {
	base := func(c *FloorCustomer) FloorProposal {
		return FloorProposal{NewOrder: true, VehicleNew: true, Kind: "carro", Plate: &Field{Value: "ABC123"}, Customer: c,
			Symptom: &Field{Value: "frenos"}}
	}
	nuevo := &FloorCustomer{Phone: &Field{Value: "3000000001"}, Name: &Field{Value: "Carlos"}}
	if err := base(nuevo).Validate(); !errors.Is(err, ErrFloorConsent) {
		t.Fatalf("sin respuesta: %v", err)
	}
	if got := base(nuevo).OrderMissing(); !slices.Contains(got, "la autorización de datos del cliente") {
		t.Fatalf("falta: %v", got)
	}
	for _, answer := range []*bool{yes(), no()} {
		c := *nuevo
		c.Consent = answer
		if err := base(&c).Validate(); err != nil {
			t.Fatalf("con respuesta %v: %v", *answer, err)
		}
	}
	// Un cliente que ya existe conserva su autorización: no se pregunta en la ficha.
	if err := base(&FloorCustomer{ID: "c1", Name: &Field{Value: "Carlos"}}).Validate(); err != nil {
		t.Fatalf("cliente existente: %v", err)
	}
}

// La respuesta la marca la persona; si cambia el celular sin volver a marcarla, es otra persona y se pregunta otra vez.
func TestHumanEditCounterConsent(t *testing.T) {
	prev := FloorProposal{NewOrder: true, VehicleNew: true, Customer: &FloorCustomer{Phone: &Field{Value: "3000000001", Origin: FromSaid},
		Name: &Field{Value: "Carlos", Origin: FromSaid}}}
	next := prev
	c := *prev.Customer
	c.Consent = yes()
	next.Customer = &c
	marked := HumanEdit(prev, next)
	if marked.Customer.Consent == nil || !*marked.Customer.Consent {
		t.Fatalf("la persona marca la respuesta: %+v", marked.Customer)
	}

	other := marked
	oc := *marked.Customer
	oc.Phone = &Field{Value: "3000000002"}
	other.Customer = &oc
	if out := HumanEdit(marked, other); out.Customer.Consent != nil {
		t.Fatalf("otro celular sin volver a marcar: %+v", out.Customer)
	}
	oc.Consent = no()
	if out := HumanEdit(marked, other); out.Customer.Consent == nil || *out.Customer.Consent {
		t.Fatalf("otro celular marcado otra vez: %+v", out.Customer)
	}

	// En un cliente que ya existía la ficha no cambia su autorización.
	exist := FloorProposal{Customer: &FloorCustomer{ID: "c1", Name: &Field{Value: "Carlos"}}}
	ex := exist
	ex.Customer = &FloorCustomer{ID: "c1", Name: &Field{Value: "Carlos"}, Consent: no()}
	if out := HumanEdit(exist, ex); out.Customer.Consent != nil {
		t.Fatalf("cliente existente: %+v", out.Customer)
	}
}
