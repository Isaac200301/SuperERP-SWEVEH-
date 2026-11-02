// Package identity es la fachada pública del módulo de identidad.
package identity

import "sweveh/internal/identity/domain"

// HasPermission decide si los roles tienen el permiso. El composition root se lo pasa a
// platform/authn para que las rutas de todos los módulos se autoricen igual (RBAC, docs/06 §6.6).
func HasPermission(roles []string, perm string) bool { return domain.HasPermission(roles, perm) }

// Permissions devuelve los permisos de unos roles.
func Permissions(roles []string) []string { return domain.Permissions(roles) }
