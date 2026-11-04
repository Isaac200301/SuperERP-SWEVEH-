package wire

import (
	"context"
	"io"
	"strings"
	"time"

	assistantapp "sweveh/internal/assistant/app"
	messagingapp "sweveh/internal/messaging/app"
	"sweveh/internal/shared"
)

// Lo que el asesor de WhatsApp (ADR-0029) consulta en otros módulos, por sus fachadas.

var dayNames = map[string]string{"lun": "lunes", "mar": "martes", "mie": "miércoles", "jue": "jueves", "vie": "viernes", "sab": "sábado", "dom": "domingo"}

func (t *taller) Profile(ctx context.Context, tallerID shared.ID) (assistantapp.WorkshopProfile, error) {
	st, err := t.s.M.Admin.Get(ctx, tallerID)
	if err != nil {
		return assistantapp.WorkshopProfile{}, err
	}
	var hours []string
	for _, k := range []string{"lun", "mar", "mie", "jue", "vie", "sab", "dom"} {
		if sl := st.Schedule[k]; sl != nil {
			hours = append(hours, dayNames[k]+" "+sl.Opens+" a "+sl.Closes)
		}
	}
	return assistantapp.WorkshopProfile{Name: st.Name, City: st.Profile.City, Attends: st.Profile.Attends, Specialties: st.Profile.Specialties,
		Hours: strings.Join(hours, ", ")}, nil
}

func (t *taller) Vehicles(ctx context.Context, tallerID, customerID shared.ID) ([]assistantapp.Vehicle, error) {
	var out []assistantapp.Vehicle
	err := t.s.M.Tx.Do(ctx, func(ctx context.Context) error {
		vs, err := t.s.M.VehDir.ByCustomer(ctx, tallerID, customerID)
		for _, v := range vs {
			desc := strings.TrimSpace(v.Kind + " " + deref(v.Brand) + " " + deref(v.Line))
			out = append(out, assistantapp.Vehicle{Plate: v.Plate, Kind: v.Kind, Description: "(" + desc + ")"})
		}
		return err
	})
	return out, err
}

// History son los últimos mensajes de texto antes del actual, en orden. Las respuestas a botones van como su título.
func (t *taller) History(ctx context.Context, tallerID, convID shared.ID, before time.Time, limit int) ([]assistantapp.Turn, error) {
	_, msgs, err := t.s.Messaging.Messages(ctx, tallerID, convID)
	if err != nil {
		return nil, err
	}
	var out []assistantapp.Turn
	for _, m := range msgs {
		if !m.CreatedAt.Before(before) || strings.TrimSpace(m.Body) == "" || m.State == "fallido" {
			continue
		}
		out = append(out, assistantapp.Turn{FromCustomer: m.Direction == "entrante", Text: m.Body, At: m.CreatedAt})
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (t *taller) Showcase(ctx context.Context, tallerID shared.ID, q string) ([]assistantapp.Product, error) {
	ps, err := t.s.M.Inventory.Showcase(ctx, tallerID, q, 15)
	out := make([]assistantapp.Product, 0, len(ps))
	for _, p := range ps {
		out = append(out, assistantapp.Product{ID: p.ID, Ref: p.Ref, Name: p.Name, Brand: p.Brand, Kind: p.Kind, Price: p.Price, Stock: p.Stock,
			HasPhoto: p.PhotoKey != ""})
	}
	return out, err
}

func (t *taller) SendMedia(ctx context.Context, tallerID shared.ID, phone, caption, ref string) (bool, error) {
	auto := "AU-04"
	if strings.TrimSpace(caption) == "" {
		caption = "Foto"
	}
	_, err := t.s.Messaging.Enqueue(ctx, messagingapp.Outgoing{TallerID: tallerID, Phone: phone, Text: caption, Origin: "agente",
		Automation: &auto, Media: ref})
	if err == messagingapp.ErrOutsideWindow {
		return false, nil
	}
	return err == nil, err
}

// media resuelve las fotos que envía el canal: "repuesto:<id>" es la foto de la vitrina (ADR-0029).
type media struct{ s *Services }

func (m media) Media(ctx context.Context, tallerID shared.ID, ref string) (messagingapp.Image, error) {
	id, err := shared.ParseID(strings.TrimPrefix(ref, "repuesto:"))
	if err != nil || !strings.HasPrefix(ref, "repuesto:") {
		return messagingapp.Image{}, shared.NotFound("foto_no_encontrada", "No encontramos la foto.")
	}
	rc, mime, err := m.s.M.Inventory.OpenPhoto(ctx, tallerID, id)
	if err != nil {
		return messagingapp.Image{}, err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, 5<<20))
	return messagingapp.Image{Data: data, MIME: mime}, err
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
