package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// R11 (V-15, V-17, HU-86, R-10): un cliente nuevo del mostrador no se guarda sin su respuesta a la autorización de
// datos. Si dice que no, se crea igual con la autorización rechazada (HU-86, criterio 2). La respuesta, su prueba
// (texto, versión, canal y usuario), la bitácora y el evento quedan en la misma transacción que el cliente. Datos
// Ficticio.
func TestCustomers_CounterConsentOnCreate(t *testing.T) {
	e := newPINEnv(t)
	c := e.registerPIN("3105551201", "Servitaller El Pistón")
	taller := tallerOf(t, c)

	// Sin respuesta: 400 y no queda nada del cliente.
	r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001202", "nombre": "Cliente Sin Respuesta"})
	if r.status != http.StatusBadRequest || r.str("code") != "autorizacion_requerida" {
		t.Fatalf("sin autorización: %d %v", r.status, r.body)
	}
	if s := c.do("GET", "/api/v1/customers/by-phone/3000001202", nil).status; s != http.StatusNotFound {
		t.Fatalf("se guardó el cliente sin autorización: %d", s)
	}
	// "autorizacion" sin "acepta" tampoco es una respuesta: el silencio no es autorización.
	if r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001202", "nombre": "Cliente", "autorizacion": map[string]any{}}); r.str("code") != "autorizacion_requerida" {
		t.Fatalf("autorización vacía: %d %v", r.status, r.body)
	}
	// Los errores de los datos salen antes que el de la autorización.
	if r := c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001202", "nombre": " "}); r.str("code") != "nombre_requerido" {
		t.Fatalf("nombre vacío: %v", r.body)
	}

	// Autoriza: el cliente queda autorizado con su prueba.
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001203", "nombre": "Cliente Autoriza",
		"autorizacion": map[string]any{"acepta": true}}))
	si := nested(r.body, "cliente", "id").(string)
	if nested(r.body, "cliente", "autorizacionEstado") != "autorizada" {
		t.Fatalf("autoriza: %v", r.body)
	}
	// No autoriza: se crea igual, marcado.
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001204", "nombre": "Cliente No Autoriza",
		"autorizacion": map[string]any{"acepta": false}}))
	no := nested(r.body, "cliente", "id").(string)
	if nested(r.body, "cliente", "autorizacionEstado") != "rechazada" {
		t.Fatalf("no autoriza: %v", r.body)
	}
	var n int
	if err := e.asTaller(taller, `SELECT count(*) FROM customers.autorizacion WHERE cliente_id IN ('`+si+`', '`+no+`')
		AND canal = 'mostrador' AND usuario_id IS NOT NULL AND version_politica = 'autorizacion-cliente-borrador-2026-11-01'
		AND texto LIKE 'Autorizo al taller a tratar mis datos%'`, &n); err != nil || n != 2 {
		t.Fatalf("prueba de la autorización: %d %v", n, err)
	}
	if err := e.asTaller(taller, `SELECT count(*) FROM audit.evento WHERE accion = 'cliente.autorizacion' AND despues->>'canal' = 'mostrador'`, &n); err != nil || n != 2 {
		t.Fatalf("bitácora: %d %v", n, err)
	}
	if err := e.db.Worker.QueryRow(context.Background(), `SELECT count(*) FROM outbox.evento WHERE tipo = 'customers.autorizacion'
		AND taller_id = $1`, taller).Scan(&n); err != nil || n != 2 {
		t.Fatalf("evento: %d %v", n, err)
	}

	existingAndIsolation(t, e, c, si)
}

// existingAndIsolation sigue la prueba de arriba: el cliente que ya existía, el log y el otro taller.
func existingAndIsolation(t *testing.T, e *env, c *client, si string) {
	t.Helper()
	// Un cliente que ya existe se devuelve sin pedir la respuesta otra vez, y su autorización no cambia.
	r := c.must(http.StatusOK, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001204", "nombre": "Otro nombre",
		"autorizacion": map[string]any{"acepta": true}}))
	if r.body["existente"] != true || nested(r.body, "cliente", "autorizacionEstado") != "rechazada" {
		t.Fatalf("existente: %v", r.body)
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001203", "nombre": "Cliente Autoriza"}))

	// El celular no llega completo al log, ni siquiera por la ruta de búsqueda (R-03 punto 6, docs/06 §10).
	c.must(http.StatusOK, c.do("GET", "/api/v1/customers/by-phone/3000001203", nil))
	if strings.Contains(e.logs.String(), "3000001203") || !strings.Contains(e.logs.String(), "by-phone/********03") {
		t.Fatal("el celular de la ruta /customers/by-phone va enmascarado al log")
	}

	// Otro taller no ve la autorización de este (CA-10).
	other := e.registerPIN("3105551205", "Otro Taller")
	if s := other.do("GET", "/api/v1/customers/"+si, nil).status; s != http.StatusNotFound {
		t.Fatalf("otro taller ve el cliente: %d", s)
	}
}

// R11 (V-15): con FEATURE_CUSTOMER_TOOLS apagada no existen "Posibles repetidos" (HU-15) ni la importación de
// clientes (HU-92); la de repuestos (HU-37) sigue. La copia y el borrado de datos (HU-87) quedan solo para el dueño.
func TestFeatures_CustomerToolsOff(t *testing.T) {
	e := newPINEnv(t)
	owner := e.registerPIN("3105551211", "Servitaller El Pistón")
	me := owner.must(http.StatusOK, owner.do("GET", "/api/v1/me", nil))
	if f := nested(me.body, "funciones").(map[string]any); f["herramientasClientes"] != false {
		t.Fatalf("funciones: %v", f)
	}
	if s := owner.do("GET", "/api/v1/customers/duplicates", nil).status; s != http.StatusNotFound {
		t.Fatalf("repetidos: %d", s)
	}
	for _, path := range []string{"/api/v1/customers/merge", "/api/v1/vehicles/import"} {
		if s := owner.do("POST", path, map[string]any{}).status; s != http.StatusNotFound && s != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s con la bandera apagada: %d", path, s)
		}
	}
	if s := owner.do("POST", "/api/v1/parts/import", map[string]any{}).status; s == http.StatusNotFound || s == http.StatusMethodNotAllowed {
		t.Fatalf("la importación de repuestos no depende de la bandera: %d", s)
	}

	// La copia y el borrado siguen para el dueño, y el mecánico (que ve todo lo demás, V-05) recibe 403.
	cid := nested(owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"celular": "3000001212",
		"nombre": "Cliente Datos", "autorizacion": acepta})).body, "cliente", "id").(string)
	owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/"+cid+"/export", nil))
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3000001213", "nombre": "Jhon Cardona", "rol": "mecanico", "pin": "4826"}))
	mec, lr := e.loginPIN("3000001213", "4826")
	mec.must(http.StatusOK, lr)
	mec.must(http.StatusNoContent, mec.do("POST", "/api/v1/auth/pin", map[string]any{"pin": "7391"}))
	for _, r := range []resp{mec.do("GET", "/api/v1/customers/"+cid+"/export", nil), mec.do("POST", "/api/v1/customers/"+cid+"/anonymize", nil)} {
		if r.status != http.StatusForbidden {
			t.Fatalf("el mecánico atiende solicitudes de datos: %d %v", r.status, r.body)
		}
	}
	owner.must(http.StatusOK, owner.do("POST", "/api/v1/customers/"+cid+"/anonymize", nil))
}
