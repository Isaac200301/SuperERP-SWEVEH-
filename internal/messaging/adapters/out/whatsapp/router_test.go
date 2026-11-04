package whatsapp_test

import (
	"context"
	"strings"
	"testing"

	"sweveh/internal/messaging/adapters/out/whatsapp"
	"sweveh/internal/messaging/adapters/out/whatsapp/local"
	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

// provider cuenta los envíos que le llegan.
type provider struct{ n int }

func (p *provider) SendText(context.Context, string, string, string) (string, error) {
	p.n++
	return "wamid.P", nil
}
func (p *provider) SendButtons(context.Context, string, string, string, []domain.Button) (string, error) {
	p.n++
	return "wamid.P", nil
}
func (p *provider) SendTemplate(context.Context, string, string, app.Template) (string, error) {
	p.n++
	return "wamid.P", nil
}
func (p *provider) SendImage(context.Context, string, string, app.Image) (string, error) {
	p.n++
	return "wamid.P", nil
}

func TestRouter_SimulatorNumbersStayLocal(t *testing.T) {
	ctx := context.Background()
	p := &provider{}
	r := whatsapp.Router{Provider: p, Local: local.Channel{}}
	btn := []domain.Button{{ID: "a", Title: "A"}}

	for _, send := range []func(pnid string) (string, error){
		func(pnid string) (string, error) { return r.SendText(ctx, pnid, "+57300", "hola") },
		func(pnid string) (string, error) { return r.SendButtons(ctx, pnid, "+57300", "hola", btn) },
		func(pnid string) (string, error) { return r.SendTemplate(ctx, pnid, "+57300", app.Template{Name: "x"}) },
		func(pnid string) (string, error) { return r.SendImage(ctx, pnid, "+57300", app.Image{}) },
	} {
		if id, err := send(local.Prefix + "taller"); err != nil || !strings.HasPrefix(id, local.Prefix) {
			t.Fatalf("simulador: %q %v", id, err)
		}
		if id, err := send("PNID-REAL"); err != nil || id != "wamid.P" {
			t.Fatalf("proveedor: %q %v", id, err)
		}
	}
	if p.n != 4 {
		t.Fatalf("al proveedor llegaron %d envíos; esperaba 4", p.n)
	}
	// Sin proveedor configurado todo va al simulador.
	if id, _ := (whatsapp.Router{Local: local.Channel{}}).SendText(ctx, "PNID-REAL", "+57300", "hola"); !strings.HasPrefix(id, local.Prefix) {
		t.Fatalf("sin proveedor: %q", id)
	}
	// El simulador también respeta el límite de botones (DR-08).
	four := []domain.Button{{ID: "a", Title: "a"}, {ID: "b", Title: "b"}, {ID: "c", Title: "c"}, {ID: "d", Title: "d"}}
	if _, err := r.SendButtons(ctx, local.Prefix+"taller", "+57300", "x", four); err == nil {
		t.Fatal("4 botones en el simulador")
	}
}
