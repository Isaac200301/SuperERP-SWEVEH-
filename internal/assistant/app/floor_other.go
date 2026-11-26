package app

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/shared"
)

// Otro vehículo en el chat de una orden (decisión de Pablo, 26-nov). El asistente no abre otra orden ni guarda nada:
// deja los datos de ese vehículo en un borrador aparte (propuesta.otraOrden) y la pantalla los muestra en una tarjeta.
// La persona decide. Con "Abrir orden nueva con estos datos" el código crea una conversación nueva cuya ficha es ese
// borrador, con sus fotos; allá la persona revisa lo que falta y toca Guardar. El asistente no tiene memoria entre
// conversaciones: lo que pasa de una a otra lo lleva el código como datos, no la conversación entera.

// ErrFloorNoOther: la conversación ya no tiene otro vehículo por abrir (se abrió o se descartó).
var ErrFloorNoOther = shared.Conflict("sin_otra_orden", "Ya no hay otro vehículo por abrir en esta conversación.")

// OpenOtherCmd es tocar "Abrir orden nueva con estos datos".
type OpenOtherCmd struct {
	TallerID, ChatID shared.ID
	Actor            shared.Actor
	Key              string
	Version          int
}

// OpenOther crea la conversación nueva con el borrador del otro vehículo y lo quita del chat de la orden, en una
// transacción. Con la misma Idempotency-Key en el mismo chat devuelve la misma conversación (doble toque o
// reintento): su id sale del chat y de la clave, así una clave repetida en otro chat no devuelve una ajena.
func (f *Floor) OpenOther(ctx context.Context, c OpenOtherCmd) (FloorChat, error) {
	key := strings.TrimSpace(c.Key)
	if len(key) < 8 || len(key) > 100 {
		return FloorChat{}, ErrFloorKey
	}
	newID := shared.NameID(c.TallerID, "otra-orden:"+c.ChatID.String()+":"+key)
	var out FloorChat
	err := f.uow.Do(ctx, func(ctx context.Context) error {
		if done, ok, err := f.openedOther(ctx, c.TallerID, newID); err != nil || ok {
			out = done
			return err
		}
		var err error
		out, err = f.openOther(ctx, c, newID)
		return err
	})
	if err != nil {
		// Dos toques a la vez: el segundo choca con la conversación del primero; si ya existe, es la misma.
		var done FloorChat
		var ok bool
		if rerr := f.uow.Do(ctx, func(ctx context.Context) error {
			var err error
			done, ok, err = f.openedOther(ctx, c.TallerID, newID)
			return err
		}); rerr == nil && ok {
			return done, nil
		}
	}
	return out, err
}

func (f *Floor) openedOther(ctx context.Context, tallerID, id shared.ID) (FloorChat, bool, error) {
	c, err := f.repo.Chat(ctx, tallerID, id)
	switch {
	case errors.Is(err, ErrFloorChat):
		return FloorChat{}, false, nil
	case err != nil:
		return FloorChat{}, false, err
	}
	return c, true, nil
}

func (f *Floor) openOther(ctx context.Context, c OpenOtherCmd, newID shared.ID) (FloorChat, error) {
	src, err := f.repo.Chat(ctx, c.TallerID, c.ChatID) // RLS y taller de la sesión: otro taller ve 404
	if err != nil {
		return FloorChat{}, err
	}
	if src.Version != c.Version {
		return FloorChat{}, ErrFloorVersion
	}
	d := src.Proposal.OtherOrder
	if d == nil || d.Plate == nil {
		return FloorChat{}, ErrFloorNoOther
	}
	// Lo que el taller sabe hoy de la placa manda (pudo cambiar desde que se armó el borrador).
	v, err := f.shop.Vehicle(ctx, c.TallerID, d.Plate.Value)
	if err != nil {
		return FloorChat{}, err
	}
	np := cloneProposal(*d)
	plate, customer, kind := np.Plate, np.Customer, np.Kind
	open := applyVehicle(&np, v)
	np.Plate = plate
	if v == nil { // vehículo nuevo: se quedan el tipo y el cliente que dijo la persona
		np.Customer, np.Kind = customer, kind
	}
	if open != nil && np.Symptom != nil { // ya tiene orden abierta: lo que trae va como nota de esa orden
		np.Notes = append(np.Notes, *np.Symptom)
		np.Symptom = nil
	}
	now := f.clock.Now()
	nc := FloorChat{ID: newID, TallerID: c.TallerID, UserID: c.Actor.ID, OrderID: open, Proposal: np, Version: 1, CreatedAt: now, UpdatedAt: now}
	intro := FloorMessage{ID: shared.NameID(c.TallerID, "otra-orden-msg:"+newID.String()), ChatID: newID, Role: "asistente",
		Text: otherIntro(src.Proposal, np), CreatedAt: now}
	for _, ph := range np.Photos { // la foto se muestra en esta conversación porque va en un mensaje suyo
		intro.Attachments = append(intro.Attachments, FloorAttachment{Kind: "foto", Ref: ph.Ref})
	}
	snap := np
	intro.Proposal = &snap
	if err := f.repo.InsertChat(ctx, nc); err != nil {
		return FloorChat{}, err
	}
	if err := f.repo.InsertMessage(ctx, c.TallerID, intro); err != nil {
		return FloorChat{}, err
	}
	src.Proposal.OtherOrder = nil // abierta allá: aquí ya no hay tarjeta (las fotos se fueron con ella)
	src.Version, src.UpdatedAt = c.Version+1, now
	ok, err := f.repo.UpdateChat(ctx, src, c.Version)
	if err != nil {
		return FloorChat{}, err
	}
	if !ok {
		return FloorChat{}, ErrFloorVersion
	}
	return nc, nil
}

// otherIntro es el primer mensaje de la conversación nueva: de dónde vienen los datos y qué falta. Lo escribe el
// código, no la IA.
func otherIntro(from, p domain.FloorProposal) string {
	var b strings.Builder
	b.WriteString("Traje lo que me contaste de " + p.Plate.Value)
	if from.OrderCode != "" {
		b.WriteString(" en la conversación de la orden " + from.OrderCode)
	}
	b.WriteString(".")
	var have []string
	if p.Customer != nil && p.Customer.Name != nil {
		have = append(have, "cliente "+p.Customer.Name.Value)
	}
	if p.Customer != nil && p.Customer.Phone != nil {
		have = append(have, "celular "+p.Customer.Phone.Value)
	}
	if p.Symptom != nil {
		have = append(have, "qué trae: "+p.Symptom.Value)
	}
	if n := len(p.Photos); n == 1 {
		have = append(have, "1 foto")
	} else if n > 1 {
		have = append(have, strconv.Itoa(n)+" fotos")
	}
	if len(have) > 0 {
		b.WriteString(" Tengo: " + strings.Join(have, ", ") + ".")
	}
	if p.OrderID != "" {
		b.WriteString(" Ese vehículo ya tiene la orden " + p.OrderCode + " abierta: seguimos en esa.")
	} else if miss := p.OrderMissing(); len(miss) > 0 {
		b.WriteString(" Falta " + joinY(miss) + ".")
	}
	b.WriteString(" Nada está guardado todavía: revisa la ficha y toca Guardar cuando esté bien.")
	return b.String()
}
