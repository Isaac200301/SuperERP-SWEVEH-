package server_test

// Caminos de error de la rebanada: lo que ve la persona cuando algo sale mal tiene que
// ser claro (DR-06) y la sesión tiene que vencer cuando debe (docs/06 §6.3).

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestOTP_ErrorPaths(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	phone := "3105550201"

	// Sin haber pedido código.
	r := c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": "123456"})
	if r.status != http.StatusBadRequest || r.str("code") != "codigo_inexistente" {
		t.Fatalf("sin código: %d %v", r.status, r.body)
	}
	// Formato de código y de celular.
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": "12ab"})
	if r.status != http.StatusBadRequest || r.str("code") != "codigo_formato" || r.str("field") != "codigo" {
		t.Fatalf("formato de código: %d %v", r.status, r.body)
	}
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": "123", "codigo": "123456"})
	if r.status != http.StatusBadRequest || r.str("code") != "celular_invalido" {
		t.Fatalf("celular inválido: %d %v", r.status, r.body)
	}
	// HU-01 criterio 2: código vencido a los 10 minutos.
	c.must(http.StatusAccepted, c.do("POST", "/api/v1/auth/otp", map[string]any{"celular": phone}))
	code := e.otp.Last("+57" + phone)
	e.clk.Advance(10 * time.Minute)
	r = c.do("POST", "/api/v1/auth/otp/verify", map[string]any{"celular": phone, "codigo": code, "nombre": "X", "autorizacionDatos": true})
	if r.status != http.StatusBadRequest || r.str("code") != "codigo_vencido" {
		t.Fatalf("código vencido: %d %v", r.status, r.body)
	}
	// JSON malformado.
	r = c.do("POST", "/api/v1/auth/otp", "esto no es un objeto")
	if r.status != http.StatusBadRequest || r.str("code") != "json_invalido" {
		t.Fatalf("json inválido: %d %v", r.status, r.body)
	}
}

func TestSession_AccessExpiresAndRefreshRenews(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550202", "Dueña G")
	c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))

	// El token de acceso dura 10 minutos.
	e.clk.Advance(11 * time.Minute)
	if r := c.do("GET", "/api/v1/me", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("acceso vencido: %d", r.status)
	}
	// El refresh sigue vivo y entrega un acceso nuevo.
	r := c.must(http.StatusOK, c.do("POST", "/api/v1/auth/refresh", nil))
	if nested(r.body, "usuario", "nombre") != "Dueña G" {
		t.Fatalf("refresh: %v", r.body)
	}
	c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))

	// El refresh vence a los 14 días sin uso.
	e.clk.Advance(15 * 24 * time.Hour)
	if r := c.do("POST", "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("refresh vencido: %d %v", r.status, r.body)
	}
}

func TestSession_LogoutRevokesRefresh(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550203", "Dueño H")
	thief := e.client()
	copyCookies(t, c, thief)
	c.must(http.StatusNoContent, c.do("POST", "/api/v1/auth/logout", nil))
	if r := thief.do("POST", "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("refresh después del cierre de sesión: %d", r.status)
	}
	// Sin cookie de refresh.
	anon := e.client()
	if r := anon.do("POST", "/api/v1/auth/refresh", nil); r.status != http.StatusUnauthorized {
		t.Fatalf("refresh sin cookie: %d", r.status)
	}
}

func TestCustomersAndVehicles_ErrorPaths(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550204", "Dueño I")
	cases := []struct {
		method, path string
		body         any
		status       int
		code         string
	}{
		{"POST", "/api/v1/customers", map[string]any{"celular": "12345", "nombre": "X"}, 400, "celular_invalido"},
		{"POST", "/api/v1/customers", map[string]any{"celular": "3005550301", "nombre": "  "}, 400, "nombre_requerido"},
		{"POST", "/api/v1/customers", map[string]any{"celular": "3005550301", "nombre": "X", "correo": "malo"}, 400, "correo_invalido"},
		{"GET", "/api/v1/customers/by-phone/123", nil, 400, "celular_invalido"},
		{"GET", "/api/v1/customers/by-phone/3009999999", nil, 404, "cliente_no_encontrado"},
		{"GET", "/api/v1/customers/no-es-uuid", nil, 404, "cliente_no_encontrado"},
		{"POST", "/api/v1/vehicles", map[string]any{"placa": "KLM482"}, 400, "cliente_requerido"},
		{"POST", "/api/v1/vehicles", map[string]any{"placa": "KLM482", "clienteId": "no-es-uuid"}, 400, "cliente_requerido"},
		{"POST", "/api/v1/vehicles", map[string]any{"placa": "##"}, 400, "placa_invalida"},
		{"POST", "/api/v1/vehicles", map[string]any{"placa": "KLM482", "cilindraje": "mucho"}, 400, "cilindraje_invalido"},
		{"GET", "/api/v1/vehicles/by-plate/ZZZ999", nil, 404, "vehiculo_no_encontrado"},
		{"GET", "/api/v1/vehicles/no-es-uuid", nil, 404, "vehiculo_no_encontrado"},
		{"PATCH", "/api/v1/vehicles/no-es-uuid/engine", map[string]any{"turbo": true}, 404, "vehiculo_no_encontrado"},
		{"POST", "/api/v1/orders", map[string]any{"placa": "ZZZ999", "sintoma": "x"}, 404, "vehiculo_no_encontrado"},
		{"POST", "/api/v1/orders", map[string]any{"vehiculoId": "no-es-uuid", "sintoma": "x"}, 404, "vehiculo_no_encontrado"},
		{"GET", "/api/v1/orders/no-es-uuid", nil, 404, "orden_no_encontrada"},
		{"GET", "/api/v1/orders?cursor=abc!", nil, 400, "cursor_invalido"},
		{"GET", "/api/v1/orders?cursor=" + "c2luLXNlcGFyYWRvcg", nil, 400, "cursor_invalido"},
		{"GET", "/api/v1/no-existe", nil, 404, "ruta_no_encontrada"},
	}
	for _, tc := range cases {
		r := c.do(tc.method, tc.path, tc.body)
		if r.status != tc.status || r.str("code") != tc.code {
			t.Errorf("%s %s: %d %v; quería %d %s", tc.method, tc.path, r.status, r.str("code"), tc.status, tc.code)
		}
	}
}

func TestOrders_Pagination(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550205", "Dueño J")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550302", "nombre": "Cliente"}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "JNX907", "clienteId": nested(r.body, "cliente", "id")}))
	for i := 0; i < 5; i++ {
		e.clk.Advance(time.Second)
		c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "JNX907", "sintoma": "x", "confirmarDuplicada": true}))
	}
	seen := map[string]bool{}
	path := "/api/v1/orders?limite=2"
	for pages := 0; pages < 5; pages++ {
		p := c.must(http.StatusOK, c.do("GET", path, nil))
		for _, o := range p.body["ordenes"].([]any) {
			seen[o.(map[string]any)["codigo"].(string)] = true
		}
		next, _ := p.body["siguiente"].(string)
		if next == "" {
			break
		}
		path = "/api/v1/orders?limite=2&cursor=" + next
	}
	if len(seen) != 5 {
		t.Fatalf("la paginación devolvió %d órdenes distintas, esperaba 5: %v", len(seen), seen)
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func copyCookies(t *testing.T, from, to *client) {
	t.Helper()
	for _, p := range []string{"/", "/api/v1/auth/refresh"} {
		u := mustURL(t, from.e.srv.URL+p)
		to.jar.SetCookies(u, from.jar.Cookies(u))
	}
}
