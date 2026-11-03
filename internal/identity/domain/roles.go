package domain

import "slices"

// Role es el rol de un usuario dentro de un taller.
type Role string

const (
	RoleOwner     Role = "dueno"
	RoleReception Role = "recepcion"
	RoleMechanic  Role = "mecanico"
)

// Permisos `recurso:accion` (docs/06 §6.6, docs/08 §3). El código pregunta por el permiso, nunca por el rol.
const (
	PermOrderCreate    = "order:create"
	PermOrderRead      = "order:read"
	PermCustomerCreate = "customer:create"
	PermCustomerRead   = "customer:read_all"
	PermVehicleCreate  = "vehicle:create"
	PermVehicleUpdate  = "vehicle:update"
	PermVehicleRead    = "vehicle:read"
)

// Lo que todos los roles del taller pueden hacer en el trabajo diario.
var common = []string{
	"order:create", "order:read", "order:update_state", "order:diagnose", "evidence:upload",
	"customer:create", "vehicle:create", "vehicle:update", "vehicle:read",
	"service:read", "part:read", "part:return",
	"quote:read", "quote:edit", "quote:send", // Q-10: por defecto el mecánico envía la cotización
}

// Front office: recepción y dueño (clientes completos, dinero, bandeja, conversaciones).
var frontOffice = []string{
	"order:cancel", "order:assign", "customer:read_all", "vehicle:change_owner", "quote:register_response",
	"money:read", "inbox:manage", "conversation:read",
}

// Solo el dueño (configuración, costos, KPIs, usuarios, bitácora).
var ownerOnly = []string{
	"order:override_approval", "service:manage", "part:manage", "part:cost", "workshop:manage", "user:manage",
	"kpi:read", "audit:read", "automation:manage", "import:run", "data_request:manage",
}

// rolePermissions: el mecánico NO ve costos, totales de dinero ni KPIs (RF-21, HU-13, HU-81).
// Difiere de docs/06 §6.6 en que el mecánico abre órdenes y registra clientes y vehículos (HU-09, HU-16; Q-30).
var rolePermissions = map[Role][]string{
	RoleOwner:     concat(common, frontOffice, ownerOnly),
	RoleReception: concat(common, frontOffice),
	RoleMechanic:  concat(common),
}

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// ValidRole indica si el texto es un rol conocido.
func ValidRole(r string) bool {
	_, ok := rolePermissions[Role(r)]
	return ok
}

// HasPermission indica si alguno de los roles otorga el permiso. Deny by default.
func HasPermission(roles []string, perm string) bool {
	for _, r := range roles {
		if slices.Contains(rolePermissions[Role(r)], perm) {
			return true
		}
	}
	return false
}

// Permissions devuelve los permisos de los roles (el frontend solo los usa para ocultar botones).
func Permissions(roles []string) []string {
	var out []string
	for _, r := range roles {
		for _, p := range rolePermissions[Role(r)] {
			if !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	return out
}
