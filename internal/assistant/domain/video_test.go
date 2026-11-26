package domain

import (
	"strings"
	"testing"
)

// La guía de lados del video (R9, HU-108): marca lo que el modelo vio, los lados con foto y lo ya guardado.
func TestGuide(t *testing.T) {
	p := FloorProposal{OrderID: "o1", OrderState: "por_recibir", Plate: &Field{Value: "ABC123", Origin: FromWorkshop}}
	p.MarkSeen("placa", "tablero", "placa", "techo") // repetido y fuera de la guía no cuentan
	p.Photos = []FloorPhoto{{Ref: "t/1", Side: "frente", Origin: FromVideo}}
	g := p.Guide(Saved{Sides: []string{"izquierdo"}})
	seen := map[string]bool{}
	for _, s := range g {
		seen[s.Key] = s.Seen
	}
	if len(g) != 6 || !seen["placa"] || !seen["tablero"] || !seen["frente"] || !seen["izquierdo"] || seen["trasera"] || seen["derecho"] {
		t.Fatalf("guía: %+v", g)
	}
	if len(p.Seen) != 2 {
		t.Fatalf("vistos sin repetir ni claves ajenas: %v", p.Seen)
	}
	if got := GuideMissing(g); got != "No vi la parte trasera ni el lado derecho. Graba otro video o dime cómo están." {
		t.Fatalf("falta: %q", got)
	}
	if pr := p.Progress(Saved{}); len(pr.Guide) != 6 {
		t.Fatalf("la ficha en vivo lleva la guía: %+v", pr)
	}
	if pr := (FloorProposal{OrderID: "o1", OrderState: "en_reparacion"}).Progress(Saved{}); pr.Guide != nil {
		t.Fatalf("sin recepción no hay guía: %+v", pr)
	}
}

func TestGuideMissing(t *testing.T) {
	all := make([]GuideStep, 0, 6)
	for _, k := range GuideKeys() {
		all = append(all, GuideStep{Key: k, Seen: true})
	}
	if GuideMissing(all) != "" {
		t.Fatal("con todo visto no falta nada")
	}
	all[5].Seen = false
	if got := GuideMissing(all); !strings.HasPrefix(got, "No vi el lado derecho.") {
		t.Fatalf("uno: %q", got)
	}
}

// Lo que vio el modelo no se cambia desde la ficha en vivo.
func TestHumanEditKeepsSeen(t *testing.T) {
	prev := FloorProposal{OrderID: "o1", Seen: []string{"placa"}}
	out := HumanEdit(prev, FloorProposal{OrderID: "o1", Seen: []string{"placa", "frente", "trasera"}})
	if len(out.Seen) != 1 {
		t.Fatalf("vistos: %v", out.Seen)
	}
}
