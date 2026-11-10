// Package whatsapp elige el canal de cada envío. Los proveedores viven en subpaquetes (meta, local y, para la
// demo, waha) y todos cumplen el puerto app.Channel; el composition root (wire) arma el Router según la
// configuración (WHATSAPP_PROVIDER).
package whatsapp

import (
	"context"
	"strings"

	"sweveh/internal/messaging/adapters/out/whatsapp/local"
	"sweveh/internal/messaging/adapters/out/whatsapp/waha"
	"sweveh/internal/messaging/app"
	"sweveh/internal/messaging/domain"
)

// Router manda los números del simulador ("local-…") al canal local, los de WAHA ("waha:…", solo demo) a WAHA y
// el resto al proveedor del taller. Sin proveedor configurado, lo que no es de WAHA va al simulador.
type Router struct {
	Provider app.Channel
	Local    app.Channel
	WAHA     app.Channel // nil si WAHA no está configurado
}

var _ app.Channel = Router{}

func (r Router) pick(pnid string) app.Channel {
	if strings.HasPrefix(pnid, waha.Prefix) && r.WAHA != nil {
		return r.WAHA
	}
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

// ResolvePhone traduce un chat que oculta el número con el canal que lo sabe hacer (WAHA); los demás lo dejan igual.
func (r Router) ResolvePhone(ctx context.Context, pnid, from string) (string, error) {
	if p, ok := r.pick(pnid).(app.PhoneResolver); ok {
		return p.ResolvePhone(ctx, pnid, from)
	}
	return from, nil
}

// FetchMedia descarga el archivo con el canal del número; el simulador no tiene archivos (HU-60).
func (r Router) FetchMedia(ctx context.Context, pnid, ref string) (app.Media, error) {
	if f, ok := r.pick(pnid).(app.MediaFetcher); ok {
		return f.FetchMedia(ctx, pnid, ref)
	}
	return app.Media{}, app.ErrNoMedia
}
