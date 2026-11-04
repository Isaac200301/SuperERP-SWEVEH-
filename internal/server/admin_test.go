package server_test

// HU-03, HU-08, HU-89 (taller), HU-80, HU-84 (usuarios), HU-12 (dueño del vehículo), HU-14 (búsqueda)
// y HU-86 (autorización en mostrador) de punta a punta. Datos Ficticio.

import (
	"net/http"
	"testing"

	"sweveh/internal/shared"
)

func TestWorkshop_OnboardingAndSchedule(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550140", "Iván Soto")

	r := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop", nil))
	if r.body["onboardingCompleto"] != false {
		t.Fatalf("taller nuevo: %v", r.body)
	}
	ok := map[string]any{"nombre": "Servitaller El Pistón", "atiende": "ambos", "especialidades": []string{"frenos_suspension", "frenos_suspension", "inventada"},
		"ciudad": "  Medellín  ", "tamano": "2a5", "contratoAceptado": true}
	for campo, malo := range map[string]any{"atiende": "barcos", "tamano": "100", "contratoAceptado": false} {
		in := map[string]any{}
		for k, v := range ok {
			in[k] = v
		}
		in[campo] = malo
		is4xx(t, "onboarding con "+campo+" inválido", c.do("POST", "/api/v1/workshop/onboarding", in))
	}
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/workshop/onboarding", ok))
	if r.str("ciudad") != "Medellín" || r.body["contratoVersion"] == nil || len(r.body["especialidades"].([]any)) != 1 {
		t.Fatalf("perfil: %v", r.body)
	}

	is4xx(t, "día que no existe", c.do("PUT", "/api/v1/workshop/schedule", map[string]any{"horario": map[string]any{"xyz": map[string]any{"abre": "08:00", "cierra": "17:00"}}}))
	is4xx(t, "cierra antes de abrir", c.do("PUT", "/api/v1/workshop/schedule", map[string]any{"horario": map[string]any{"lun": map[string]any{"abre": "17:00", "cierra": "08:00"}}}))
	r = c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/schedule", map[string]any{"horario": map[string]any{
		"lun": map[string]any{"abre": "07:30", "cierra": "17:30"}, "dom": nil,
	}}))
	if nested(r.body, "horario", "lun", "abre") != "07:30" {
		t.Fatalf("horario: %v", r.body)
	}
}

// HU-04 y HU-05: el arranque precarga servicios según lo que atiende y la primera orden puede ser de ejemplo.
func TestOnboarding_PreloadAndExampleOrder(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550141", "Marta Ríos")
	in := map[string]any{"nombre": "Motos La 80", "atiende": "moto", "especialidades": []string{"latoneria_pintura", "aire_acondicionado"},
		"ciudad": "medellin", "tamano": "solo", "contratoAceptado": true}
	r := c.must(http.StatusOK, c.do("POST", "/api/v1/workshop/onboarding", in))
	if r.str("ciudad") != "Medellin" {
		t.Errorf("ciudad normalizada: %v", r.body["ciudad"])
	}

	nombres := func() map[string]string {
		r := c.must(http.StatusOK, c.do("GET", "/api/v1/services", nil))
		m := map[string]string{}
		for _, s := range r.body["servicios"].([]any) {
			sv := s.(map[string]any)
			m[sv["nombre"].(string)] = sv["tipoVehiculo"].(string)
		}
		return m
	}
	antes := nombres()
	if antes["Latonería"] != "moto" || antes["Cambio de kit de arrastre"] != "moto" {
		t.Fatalf("la precarga debía traer latonería y mantenimiento de moto: %v", antes)
	}
	for n, tipo := range antes {
		if tipo == "carro" {
			t.Fatalf("un taller de motos no debía recibir servicios de carro: %s", n)
		}
	}
	// Repetir el arranque con una especialidad más suma solo lo nuevo, sin duplicar.
	in["especialidades"] = []string{"latoneria_pintura", "aire_acondicionado", "electrico"}
	c.must(http.StatusOK, c.do("POST", "/api/v1/workshop/onboarding", in))
	despues := nombres()
	if _, ok := despues["Revisión eléctrica"]; !ok || len(despues) != len(antes)+3 {
		t.Fatalf("debía sumar los 3 servicios de sistema eléctrico: antes %d, después %d", len(antes), len(despues))
	}
	w := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop", nil))
	if esp := w.body["especialidades"].([]any); len(esp) != 3 || esp[2] != "sistema_electrico" {
		t.Errorf("la clave vieja \"electrico\" debía guardarse como sistema_electrico: %v", esp)
	}

	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000000000", "nombre": "Cliente de ejemplo"}))
	clienteID := nested(r.body, "cliente", "id").(string)
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "EJM12A", "clienteId": clienteID}))
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "EJM12A", "sintoma": "Revisión general (ejemplo)", "ejemplo": true}))
	if r.body["ejemplo"] != true {
		t.Fatalf("la orden debía quedar marcada como ejemplo: %v", r.body)
	}
}

func TestUsers_AddDeactivateAndSwitchWorkshop(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550150", "Gloria Paz")
	other := e.register("3105550151", "Tomás Gil")

	is4xx(t, "rol inválido", owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550152", "nombre": "X", "rol": "dueno"}))
	is4xx(t, "celular inválido", owner.do("POST", "/api/v1/users", map[string]any{"celular": "123", "nombre": "X", "rol": "mecanico"}))
	r := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550152", "nombre": "Andrés Mecánico", "rol": "mecanico"}))
	mecID := r.str("id")
	// Un usuario que ya existe (dueño de otro taller) entra como recepción.
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3105550151", "nombre": "Tomás Gil", "rol": "recepcion"}))
	r = owner.must(http.StatusOK, owner.do("GET", "/api/v1/users", nil))
	if u, _ := r.body["usuarios"].([]any); len(u) != 3 {
		t.Fatalf("usuarios: %v", r.body)
	}

	// HU-84: desactivar y reactivar; al dueño ni a uno mismo.
	r = owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/users/"+mecID, map[string]any{"activo": false}))
	if r.body["activo"] != false {
		t.Fatalf("desactivar: %v", r.body)
	}
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/users/"+mecID, map[string]any{"activo": true}))
	me := owner.must(http.StatusOK, owner.do("GET", "/api/v1/me", nil))
	is4xx(t, "desactivarse a sí mismo", owner.do("PATCH", "/api/v1/users/"+nested(me.body, "usuario", "id").(string), map[string]any{"activo": false}))
	is4xx(t, "usuario que no existe", owner.do("PATCH", "/api/v1/users/"+shared.NewID().String(), map[string]any{"activo": false}))

	// Quien está en dos talleres los ve y cambia de uno a otro.
	r = other.must(http.StatusOK, other.do("GET", "/api/v1/me/workshops", nil))
	talleres, _ := r.body["talleres"].([]any)
	if len(talleres) != 2 {
		t.Fatalf("talleres: %v", r.body)
	}
	destino := tallerOf(t, owner).String()
	is4xx(t, "taller inválido", other.do("POST", "/api/v1/auth/switch", map[string]any{"tallerId": "no-es-uuid"}))
	is4xx(t, "taller ajeno", other.do("POST", "/api/v1/auth/switch", map[string]any{"tallerId": shared.NewID().String()}))
	r = other.must(http.StatusOK, other.do("POST", "/api/v1/auth/switch", map[string]any{"tallerId": destino}))
	if nested(r.body, "taller", "id") != destino {
		t.Fatalf("cambio de taller: %v", r.body)
	}
}

func TestCustomers_ListSearchConsentAndOwnerChange(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550160", "Nora Vélez")
	laura, vehID, _ := seedOrder(t, c, "3005550161", "BNM456")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550162", "nombre": "Pedro Arango"}))
	pedro := nested(r.body, "cliente", "id").(string)

	r = c.must(http.StatusOK, c.do("GET", "/api/v1/customers", nil))
	if l, _ := r.body["clientes"].([]any); len(l) != 2 {
		t.Fatalf("clientes: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/customers?q=Pedro", nil))
	if l, _ := r.body["clientes"].([]any); len(l) != 1 {
		t.Fatalf("búsqueda de clientes: %v", r.body)
	}

	// HU-86: autorización dada en el mostrador.
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/customers/"+laura+"/consent", map[string]any{"acepta": true}))
	if r.str("autorizacionEstado") != "autorizada" {
		t.Fatalf("autorización: %v", r.body)
	}
	is4xx(t, "autorización de cliente ajeno", c.do("POST", "/api/v1/customers/"+shared.NewID().String()+"/consent", map[string]any{"acepta": true}))

	// HU-14: buscar por placa trae el vehículo con su orden abierta.
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/search?q=BNM", nil))
	if v, _ := r.body["vehiculos"].([]any); len(v) != 1 || nested(v[0].(map[string]any), "ordenAbierta") == nil {
		t.Fatalf("búsqueda: %v", r.body)
	}
	c.must(http.StatusOK, c.do("GET", "/api/v1/search?q=Pedro", nil))

	// HU-12: el vehículo pasa a Pedro con control de versión.
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/customers/"+laura+"/vehicles", nil))
	if v, _ := r.body["vehiculos"].([]any); len(v) != 1 {
		t.Fatalf("vehículos del cliente: %v", r.body)
	}
	v := c.must(http.StatusOK, c.do("GET", "/api/v1/vehicles/"+vehID, nil)).body["version"]
	is4xx(t, "cliente inválido", c.do("PATCH", "/api/v1/vehicles/"+vehID+"/owner", map[string]any{"clienteId": "x", "version": v}))
	r = c.do("PATCH", "/api/v1/vehicles/"+vehID+"/owner", map[string]any{"clienteId": pedro, "version": 99})
	if r.status != http.StatusConflict {
		t.Fatalf("versión vieja: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/vehicles/"+vehID+"/owner", map[string]any{"clienteId": pedro, "version": v}))
	if r.str("clienteId") != pedro {
		t.Fatalf("cambio de dueño: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/customers/"+pedro+"/vehicles", nil))
	if l, _ := r.body["vehiculos"].([]any); len(l) != 1 {
		t.Fatalf("vehículos del dueño nuevo: %v", r.body)
	}
}
