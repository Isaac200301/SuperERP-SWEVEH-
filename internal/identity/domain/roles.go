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
	"expense:manage", // V-08: lo gastado contra lo cotizado lo anota cualquiera del equipo
	"assistant:use",  // V-11: el asistente del piso es de todo el equipo (al confirmar se revisa cada parte)
}

// Un solo panel (V-05, ADR-0038, entrega del 27-nov): todo el equipo atiende clientes, ve precios y dinero de la orden,
// da descuentos hasta el tope de Mi taller y cancela con motivo. Cambia RF-21 y HU-81 (antes el mecánico no veía dinero).
var floor = []string{
	"order:cancel", "order:assign", "customer:read_all", "vehicle:change_owner", "quote:register_response", "quote:discount",
	"money:read", "inbox:manage", "conversation:read",
}

// Los números del taller (KPI): dueño y recepción (V-05). Quien no los ve recibe "Hoy tienes pendiente" sin montos.
var numbers = []string{"kpi:read"}

// Solo el dueño: Equipo, Mi taller, la lista de servicios y precios, costos de repuestos, bitácora, automatizaciones,
// importaciones y los datos personales de los clientes (copia, borrado, unir repetidos).
var ownerOnly = []string{
	"order:override_approval", "service:manage", "part:manage", "part:cost", "workshop:manage", "user:manage",
	"audit:read", "automation:manage", "import:run", "data_request:manage", "customer:merge",
}

// rolePermissions: los 3 roles son universales y cada taller decide quién tiene cuál (V-05).
// El mecánico abre órdenes y registra clientes y vehículos (HU-09, HU-16; Q-30).
var rolePermissions = map[Role][]string{
	RoleOwner:     concat(common, floor, numbers, ownerOnly),
	RoleReception: concat(common, floor, numbers),
	RoleMechanic:  concat(common, floor),
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
