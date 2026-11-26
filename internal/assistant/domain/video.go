package domain

import (
	"slices"
	"strings"
)

// Recepción por video (R9, V-12, ADR-0042, HU-108). El celular graba hasta 90 s, pero el video completo nunca sale del
// celular (minimización, Ley 1581): llegan solo el audio y de 8 a 10 cuadros. El asistente del piso los convierte en
// la misma propuesta de R8 y la guía de lados dice qué vio y qué falta.

// FromVideo es el origen de un dato que salió del video: lo dijo la persona en el audio o se ve en un cuadro.
const FromVideo = "video"

// Límites del video.
const (
	VideoMaxSeconds = 90
	VideoMaxFrames  = 10
)

// GuideStep es un paso de la guía de lados: ✓ Placa, ✓ Tablero, ○ Lado izq.…
type GuideStep struct {
	Key   string `json:"clave"`
	Label string `json:"nombre"`
	Seen  bool   `json:"visto"`
}

// videoGuide es el orden en que se recorre el vehículo con la cámara (Figma 08). Placa y tablero no son lados con
// foto obligatoria en un carro, pero en el video se piden: la placa confirma el vehículo y el tablero da el
// kilometraje y el combustible.
var videoGuide = []GuideStep{
	{Key: "placa", Label: "Placa"}, {Key: "tablero", Label: "Tablero"}, {Key: "frente", Label: "Frente"},
	{Key: "izquierdo", Label: "Lado izq."}, {Key: "trasera", Label: "Atrás"}, {Key: "derecho", Label: "Lado der."},
}

// GuideKeys son las claves que el modelo puede reportar como vistas.
func GuideKeys() []string {
	out := make([]string, 0, len(videoGuide))
	for _, g := range videoGuide {
		out = append(out, g.Key)
	}
	return out
}

// Guide marca lo que el modelo reportó haber visto en un video, los lados que ya tienen foto en la propuesta y los que
// la orden ya tiene guardados. Lo demás queda pendiente.
func (p FloorProposal) Guide(saved Saved) []GuideStep {
	out := make([]GuideStep, len(videoGuide))
	for i, g := range videoGuide {
		g.Seen = slices.Contains(p.Seen, g.Key) || slices.Contains(saved.Sides, g.Key)
		for _, ph := range p.Photos {
			g.Seen = g.Seen || ph.Side == g.Key
		}
		out[i] = g
	}
	return out
}

// MarkSeen agrega lo que el modelo dice haber visto (solo claves de la guía, sin repetir).
func (p *FloorProposal) MarkSeen(keys ...string) {
	for _, k := range keys {
		if slices.Contains(GuideKeys(), k) && !slices.Contains(p.Seen, k) {
			p.Seen = append(p.Seen, k)
		}
	}
}

var guideWords = map[string]string{"placa": "la placa", "tablero": "el tablero", "frente": "el frente",
	"izquierdo": "el lado izquierdo", "trasera": "la parte trasera", "derecho": "el lado derecho"}

// GuideMissing es el texto fijo (no lo redacta la IA) que dice qué no se vio: "" si se vio todo.
func GuideMissing(g []GuideStep) string {
	var miss []string
	for _, s := range g {
		if !s.Seen {
			miss = append(miss, guideWords[s.Key])
		}
	}
	switch len(miss) {
	case 0:
		return ""
	case 1:
		return "No vi " + miss[0] + ". Graba otro video o dime cómo está."
	}
	return "No vi " + strings.Join(miss[:len(miss)-1], ", ") + " ni " + miss[len(miss)-1] + ". Graba otro video o dime cómo están."
}
