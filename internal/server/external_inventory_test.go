package server_test

import (
	"net/http"
	"testing"
)

// R4 (V-06, ADR-0039, HU-110): "Llevo el inventario en otro programa". Datos Ficticio.

// setExternal prende o apaga el interruptor en Mi taller → Avanzado (con la versión de los parámetros).
func setExternal(t *testing.T, c *client, on bool) {
	t.Helper()
	s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body
	s["inventarioExterno"] = on
	r := c.must(http.StatusOK, c.do("PUT", "/api/v1/workshop/settings", s))
	if r.body["inventarioExterno"] != on {
		t.Fatalf("parámetros: %v", r.body)
	}
}

func TestExternalInventory_ManualPartsNoStockMoves(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550410", "Rosa Pérez")
	// Un repuesto del inventario de antes, con existencia, y una orden que lo cotiza ANTES de apagar el inventario.
	pid := c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "PF-410", "nombre": "Pastillas", "tipoVehiculo": "carro", "costo": 40000, "precioVenta": 65000, "existencia": 5, "minimo": 1,
	})).str("id")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550411", "nombre": "Ana Ríos"}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "EXT410", "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "1400", "turbo": false, "energia": "gasolina", "transmision": "manual"}))
	id := c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "EXT410", "sintoma": "Frena mal", "estadoInicial": "recibido"})).str("id")
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+id+"/transition", map[string]any{"estado": "en_diagnostico", "version": r2v(c, id)}))
	q := "/api/v1/orders/" + id + "/quote"
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 2}))

	// Por defecto un taller que ya existía sigue con su inventario (Q-41).
	if s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body; s["inventarioExterno"] != false {
		t.Fatalf("por defecto: %v", s)
	}
	setExternal(t, c, true)

	// /parts: se lee (datos viejos) pero no se escribe.
	c.must(http.StatusOK, c.do("GET", "/api/v1/parts", nil))
	for _, w := range []struct{ method, path string }{
		{"POST", "/api/v1/parts"}, {"PATCH", "/api/v1/parts/" + pid}, {"POST", "/api/v1/parts/" + pid + "/entries"},
		{"POST", "/api/v1/parts/" + pid + "/returns"},
	} {
		if r := c.do(w.method, w.path, map[string]any{"cantidad": 1}); r.status != http.StatusConflict || r.str("code") != "inventario_externo" {
			t.Fatalf("%s %s: %d %v", w.method, w.path, r.status, r.body)
		}
	}

	// En la cotización: un repuesto del inventario ya no; a mano sí, con el valor que pone el taller.
	if r := c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 1}); r.status != http.StatusConflict || r.str("code") != "inventario_externo" {
		t.Fatalf("repuesto del inventario: %d %v", r.status, r.body)
	}
	if r := c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "descripcion": "Kit de embrague", "cantidad": 1}); r.str("code") != "precio_requerido" {
		t.Fatalf("a mano sin precio: %d %v", r.status, r.body)
	}
	if r := c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "descripcion": "Kit de embrague", "cantidad": 1, "precioUnitario": 0}); r.str("code") != "precio_repuesto" {
		t.Fatalf("a mano en cero: %d %v", r.status, r.body)
	}
	ls := lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "descripcion": "Kit de embrague", "cantidad": 1, "precioUnitario": 480000})))
	if len(ls) != 2 || ls[1]["refId"] != nil || ls[1]["precioUnitario"] != float64(480000) {
		t.Fatalf("líneas: %v", ls)
	}

	// Aprobar no descuenta existencias, ni de la línea vieja con referencia; cancelar no devuelve nada.
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	if s := partStock(t, c, "PF-410"); s != 5 {
		t.Fatalf("la existencia no debía moverse: %v", s)
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+id+"/cancel", map[string]any{"motivo": "El cliente no volvió", "version": r2v(c, id)}))
	if m := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id+"/part-movements", nil)); len(asList(m.body["repuestos"])) != 0 {
		t.Fatalf("no debía haber movimientos: %v", m.body)
	}
	if s := partStock(t, c, "PF-410"); s != 5 {
		t.Fatalf("existencia tras cancelar: %v", s)
	}

	// El cambio queda en la bitácora del taller.
	var n int
	if err := e.asTaller(tallerOf(t, c), `SELECT count(*) FROM audit.evento WHERE accion = 'taller.parametros'
		AND (despues->>'inventario_externo')::boolean`, &n); err != nil || n != 1 {
		t.Fatalf("bitácora: %d %v", n, err)
	}
}

// R-06: el interruptor es de cada taller; el otro sigue escribiendo su inventario. El paso 2 del registro lo deja
// encendido si la casilla viene marcada (Q-41).
func TestExternalInventory_PerWorkshopAndOnboarding(t *testing.T) {
	e := newEnv(t)
	a := e.register("3105550412", "Taller A")
	b := e.register("3105550413", "Taller B")
	setExternal(t, a, true)
	if r := a.do("POST", "/api/v1/parts", map[string]any{"referencia": "X1", "nombre": "X", "tipoVehiculo": "carro", "precioVenta": 1000}); r.status != http.StatusConflict {
		t.Fatalf("A: %d", r.status)
	}
	b.must(http.StatusCreated, b.do("POST", "/api/v1/parts", map[string]any{"referencia": "X1", "nombre": "X", "tipoVehiculo": "carro", "costo": 500, "precioVenta": 1000, "existencia": 1, "minimo": 0}))
	if s := b.must(http.StatusOK, b.do("GET", "/api/v1/workshop/settings", nil)).body; s["inventarioExterno"] != false {
		t.Fatalf("B no debía cambiar: %v", s)
	}

	c := e.register("3105550414", "Taller C")
	c.must(http.StatusOK, c.do("POST", "/api/v1/workshop/onboarding", map[string]any{"nombre": "Taller C", "atiende": "carro", "ciudad": "Medellín",
		"tamano": "solo", "contratoAceptado": true, "inventarioExterno": true}))
	if s := c.must(http.StatusOK, c.do("GET", "/api/v1/workshop/settings", nil)).body; s["inventarioExterno"] != true {
		t.Fatalf("el registro con la casilla marcada: %v", s)
	}
}

func asList(v any) []any { l, _ := v.([]any); return l }
