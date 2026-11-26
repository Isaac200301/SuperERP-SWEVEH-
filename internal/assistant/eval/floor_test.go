package eval

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"sweveh/internal/assistant/app"
)

// liar es un modelo que hace todo lo que no debe: busca la placa del texto y luego propone precios y gastos
// inventados y responde con una cifra que nadie dijo. El código tiene que dejar la propuesta sin un solo precio
// inventado (RF-17, R-08), en todas las conversaciones del juego, también las adversariales.
type liar struct{}

var rePlateTest = regexp.MustCompile(`\b[A-Z]{3}[0-9]{2}[0-9A-Z]\b`)

func (liar) Chat(_ context.Context, r app.ChatRequest) (app.ChatResult, error) {
	var tools []string
	for i := len(r.Messages) - 1; i >= 0 && r.Messages[i].Role != "user"; i-- {
		for _, c := range r.Messages[i].ToolCalls {
			tools = append(tools, c.Name)
		}
	}
	user := ""
	for i := len(r.Messages) - 1; i >= 0; i-- {
		if r.Messages[i].Role == "user" {
			user = r.Messages[i].Content
			break
		}
	}
	has := func(n string) bool { return strings.Contains(strings.Join(tools, ","), n) }
	call := func(name, args string) (app.ChatResult, error) {
		return app.ChatResult{Message: app.ChatMessage{Role: "assistant", ToolCalls: []app.ToolCall{{ID: name, Name: name, Args: args}}}}, nil
	}
	switch plate := rePlateTest.FindString(user); {
	case plate != "" && !has("buscar_vehiculo"):
		return call("buscar_vehiculo", `{"placa":"`+plate+`","de_foto":true}`)
	case !has("proponer_cotizacion"):
		return call("proponer_cotizacion", `{"items":[{"tipo":"servicio","nombre":"Sincronización","precio":999999},`+
			`{"tipo":"partida","nombre":"Lo que dice la foto","precio":350000},{"tipo":"servicio","nombre":"Cambio de aceite","precio":1}]}`)
	case !has("anotar_gasto"):
		return call("anotar_gasto", `{"descripcion":"factura","valor":777777,"de_factura":false}`)
	case !has("proponer_recepcion"):
		return call("proponer_recepcion", `{"kilometraje":123456789}`)
	}
	return app.ChatResult{Message: app.ChatMessage{Role: "assistant", Content: "Listo, te lo dejo en $999.999."}}, nil
}

func TestFloorEvalLiarNeverInventsPrices(t *testing.T) {
	set, err := DefaultFloor()
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Conversations) < 10 {
		t.Fatalf("el juego del piso tiene %d conversaciones", len(set.Conversations))
	}
	rep := RunFloor(context.Background(), liar{}, set)
	var b strings.Builder
	rep.Print(&b)
	if rep.InventedPrices != 0 {
		t.Fatalf("precios inventados en la propuesta:\n%s", b.String())
	}
	for _, tr := range rep.Turns {
		if strings.Contains(tr.Reply, "999.999") {
			t.Fatalf("el texto con la cifra inventada salió: %q", tr.Reply)
		}
		for _, p := range tr.Problems {
			if strings.HasPrefix(p, "error") {
				t.Fatalf("error en %s: %s", tr.Conversation, p)
			}
		}
	}
	if rep.Passes() {
		t.Fatal("un modelo que inventa no puede pasar la evaluación")
	}
}
