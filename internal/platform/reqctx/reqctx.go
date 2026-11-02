// Package reqctx lleva por el context los datos de la solicitud que salen de la
// sesión validada en el servidor: taller, usuario, roles y request_id.
// El taller_id NUNCA sale del cuerpo, la URL ni un encabezado del cliente (R-06).
package reqctx

import (
	"context"

	"sweveh/internal/shared"
)

type keyPrincipal struct{}
type keyRequestID struct{}
type keyTenant struct{}

// Principal es el usuario autenticado de una solicitud del realm taller.
type Principal struct {
	UserID    shared.ID
	TallerID  shared.ID
	SessionID shared.ID
	Roles     []string
}

// Actor devuelve el actor de auditoría correspondiente.
func (p Principal) Actor() shared.Actor {
	return shared.Actor{Type: shared.ActorUser, ID: p.UserID}
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	ctx = context.WithValue(ctx, keyPrincipal{}, p)
	return WithTenant(ctx, p.TallerID)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(keyPrincipal{}).(Principal)
	return p, ok
}

// WithTenant fija el taller de la operación. Solo lo llaman el middleware de
// autenticación y los flujos de sistema que ya resolvieron el taller (registro, worker).
func WithTenant(ctx context.Context, tallerID shared.ID) context.Context {
	return context.WithValue(ctx, keyTenant{}, tallerID)
}

func TenantFrom(ctx context.Context) (shared.ID, bool) {
	id, ok := ctx.Value(keyTenant{}).(shared.ID)
	return id, ok && id != shared.NilID
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID{}, id)
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(keyRequestID{}).(string)
	return id
}
