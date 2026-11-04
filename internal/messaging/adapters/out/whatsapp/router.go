// Package whatsapp elige el canal de cada envío. Los proveedores viven en subpaquetes (meta, local y, para la
// demo, waha) y todos cumplen el puerto app.Channel; el composition root (wire) arma el Router según la
// configuración (WHATSAPP_PROVIDER).
package whatsapp

import (
	"context"
	"strings"

	"sweveh/internal/messaging/adapters/out/whatsapp/local"
	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

// Router manda los números del simulador ("local-…") al canal local y el resto al proveedor del taller.
// Sin proveedor configurado, todo va al simulador.
type Router struct {
	Provider app.Channel
	Local    app.Channel
}

var _ app.Channel = Router{}

func (r Router) pick(pnid string) app.Channel {
	if strings.HasPrefix(pnid, local.Prefix) || r.Provider == nil {
		return r.Local
	}
	return r.Provider
}

func (r Router) SendText(ctx context.Context, pnid, to, text string) (string, error) {
	return r.pick(pnid).SendText(ctx, pnid, to, text)
}

func (r Router) SendButtons(ctx context.Context, pnid, to, text string, b []domain.Button) (string, error) {
	return r.pick(pnid).SendButtons(ctx, pnid, to, text, b)
}

func (r Router) SendTemplate(ctx context.Context, pnid, to string, t app.Template) (string, error) {
	return r.pick(pnid).SendTemplate(ctx, pnid, to, t)
}

func (r Router) SendImage(ctx context.Context, pnid, to string, img app.Image) (string, error) {
	return r.pick(pnid).SendImage(ctx, pnid, to, img)
}
