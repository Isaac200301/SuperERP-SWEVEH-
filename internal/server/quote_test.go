package server_test

// HU-07 (servicios), HU-32/35/36/38 (inventario), HU-27/29/34 (cotización) y HU-33 (descuento al aprobar)
// de punta a punta. Datos Ficticio.

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"sweveh/internal/shared"
)

// cot desenvuelve {"cotizacion": ...}, la forma de todas las respuestas de cotización.
func cot(r resp) resp {
	m, _ := r.body["cotizacion"].(map[string]any)
	r.body = m
	return r
}

func partStock(t *testing.T, c *client, ref string) float64 {
	t.Helper()
	r := c.must(http.StatusOK, c.do("GET", "/api/v1/parts?q="+ref, nil))
	l, _ := r.body["repuestos"].([]any)
	if len(l) != 1 {
		t.Fatalf("repuesto %s: %v", ref, r.body)
	}
	return l[0].(map[string]any)["existencia"].(float64)
}

func TestCatalog_ServicesAndParts(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550180", "Lucía Ramos")

	// HU-07: servicios con y sin precio, validación, versión optimista y borrado.
	is4xx(t, "tipo inválido", c.do("POST", "/api/v1/services", map[string]any{"nombre": "Lavado", "tipoVehiculo": "avion"}))
	is4xx(t, "sin nombre", c.do("POST", "/api/v1/services", map[string]any{"nombre": " ", "tipoVehiculo": "carro"}))
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Cambio de aceite", "tipoVehiculo": "carro", "precio": 80000, "duracionMin": 45}))
	sid, sv := r.str("id"), r.body["version"]
	c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Revisión eléctrica", "tipoVehiculo": "ambos"}))
	if r = c.do("PATCH", "/api/v1/services/"+sid, map[string]any{"precio": 90000, "version": 99}); r.status != http.StatusConflict {
		t.Fatalf("versión vieja: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/services/"+sid, map[string]any{"precio": 90000, "nombre": "Cambio de aceite 20W50", "activo": false, "version": sv}))
	if r.body["precio"] != float64(90000) || r.body["activo"] != false {
		t.Fatalf("servicio editado: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/services/"+sid, map[string]any{"precio": nil, "version": r.body["version"]}))
	if r.body["precio"] != nil {
		t.Fatalf("precio borrado: %v", r.body)
	}
	activos := len(c.must(http.StatusOK, c.do("GET", "/api/v1/services", nil)).body["servicios"].([]any))
	todos := len(c.must(http.StatusOK, c.do("GET", "/api/v1/services?todos=1", nil)).body["servicios"].([]any))
	if todos != activos+1 {
		t.Fatalf("activos=%d todos=%d", activos, todos)
	}
	c.must(http.StatusNoContent, c.do("DELETE", "/api/v1/services/"+sid, nil))
	is4xx(t, "borrar dos veces", c.do("DELETE", "/api/v1/services/"+sid, nil))

	// HU-32, HU-35 y HU-36: repuestos, entradas y alerta bajo el mínimo.
	is4xx(t, "precio negativo", c.do("POST", "/api/v1/parts", map[string]any{"referencia": "X", "nombre": "X", "tipoVehiculo": "carro", "precioVenta": -1}))
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "FA-10", "nombre": "Filtro de aceite", "tipoVehiculo": "carro", "costo": 12000, "precioVenta": 20000, "existencia": 1, "minimo": 2,
	}))
	pid := r.str("id")
	if r.body["costo"] != float64(12000) || r.body["bajoMinimo"] != true {
		t.Fatalf("el dueño ve el costo y la alerta: %v", r.body)
	}
	is4xx(t, "referencia repetida", c.do("POST", "/api/v1/parts", map[string]any{"referencia": "FA-10", "nombre": "Otro", "tipoVehiculo": "carro", "precioVenta": 1}))
	if l := c.must(http.StatusOK, c.do("GET", "/api/v1/parts/low", nil)).body["repuestos"].([]any); len(l) != 1 {
		t.Fatalf("bajo el mínimo: %v", l)
	}
	is4xx(t, "entrada en cero", c.do("POST", "/api/v1/parts/"+pid+"/entries", map[string]any{"cantidad": 0}))
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/parts/"+pid+"/entries", map[string]any{"cantidad": 4}))
	if r.body["existencia"] != float64(5) || r.body["bajoMinimo"] != false {
		t.Fatalf("entrada: %v", r.body)
	}
	if r = c.do("PATCH", "/api/v1/parts/"+pid, map[string]any{"precioVenta": 22000, "version": 99}); r.status != http.StatusConflict {
		t.Fatalf("versión vieja: %d %v", r.status, r.body)
	}
	v := c.must(http.StatusOK, c.do("GET", "/api/v1/parts?q=FA-10", nil)).body["repuestos"].([]any)[0].(map[string]any)["version"]
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/parts/"+pid, map[string]any{"precioVenta": 22000, "nombre": "Filtro de aceite Mann", "minimo": 1, "version": v}))
	if r.body["precioVenta"] != float64(22000) {
		t.Fatalf("repuesto editado: %v", r.body)
	}
	is4xx(t, "repuesto ajeno", c.do("PATCH", "/api/v1/parts/"+shared.NewID().String(), map[string]any{"version": 1}))
}

func TestQuote_BuildSendApproveDiscountAndReturn(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550190", "Mario Lenis")
	conPrecio := c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Alineación", "tipoVehiculo": "carro", "precio": 60000})).str("id")
	sinPrecio := c.must(http.StatusCreated, c.do("POST", "/api/v1/services", map[string]any{"nombre": "Diagnóstico eléctrico", "tipoVehiculo": "carro"})).str("id")
	pid := c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "PF-20", "nombre": "Pastillas", "tipoVehiculo": "carro", "costo": 40000, "precioVenta": 65000, "existencia": 5, "minimo": 1,
	})).str("id")

	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550191", "nombre": "Ana Ríos"}))
	cid := nested(r.body, "cliente", "id").(string)
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "WSX741", "clienteId": cid, "cilindraje": "1400", "turbo": false}))
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "WSX741", "sintoma": "Frena mal", "estadoInicial": "recibido"}))
	id := r.str("id")
	q := "/api/v1/orders/" + id + "/quote"

	// Sin cotización todavía; en Recibido no se cotiza.
	if r = c.must(http.StatusOK, c.do("GET", q, nil)); r.body["cotizacion"] != nil || len(r.body) != 1 {
		t.Fatalf("sin cotización: %v", r.body)
	}
	if r = c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": conPrecio, "cantidad": 1}); r.str("code") != "orden_no_cotizable" {
		t.Fatalf("cotizar en Recibido: %d %v", r.status, r.body)
	}
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+id+"/transition", map[string]any{"estado": "en_diagnostico", "version": r2v(c, id)}))

	// HU-27 y HU-34: precio de la lista del taller, nunca inventado.
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": conPrecio, "cantidad": 1, "precioUnitario": 1}))
	if r = c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": sinPrecio, "cantidad": 1}); r.str("code") != "precio_requerido" {
		t.Fatalf("servicio sin precio: %d %v", r.status, r.body)
	}
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": sinPrecio, "cantidad": 1, "precioUnitario": 50000}))
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 2})))
	is4xx(t, "repuesto que no existe", c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": shared.NewID().String(), "cantidad": 1}))
	is4xx(t, "ítem libre sin precio", c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Lavado", "cantidad": 1}))
	is4xx(t, "cantidad cero", c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Lavado", "cantidad": 0, "precioUnitario": 1}))
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Lavado", "cantidad": 1, "precioUnitario": 15000})))
	lineas, ok := r.body["lineas"].([]any)
	if !ok {
		t.Fatalf("sin líneas: %d %v", r.status, r.body)
	}
	if len(lineas) != 4 || r.body["total"] != float64(60000+50000+2*65000+15000) {
		t.Fatalf("cotización: %v", r.body)
	}
	if lineas[0].(map[string]any)["precioUnitario"] != float64(60000) {
		t.Fatalf("el precio debía salir de la lista, no del cliente: %v", lineas[0])
	}
	libre := lineas[3].(map[string]any)["id"].(string)
	r = cot(c.must(http.StatusOK, c.do("DELETE", q+"/lines/"+libre, nil)))
	if len(r.body["lineas"].([]any)) != 3 {
		t.Fatalf("quitar ítem: %v", r.body)
	}
	is4xx(t, "quitar ítem que no existe", c.do("DELETE", q+"/lines/"+shared.NewID().String(), nil))
	is4xx(t, "responder sin enviar", c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "telefono"}))

	// Enviar: la orden pasa a Esperando aprobación y la cotización ya no se edita.
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/send", nil)))
	if r.str("estado") != "enviada" || r.body["enviadaEn"] == nil {
		t.Fatalf("enviada: %v", r.body)
	}
	if o := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id, nil)); o.str("estado") != "esperando_aprobacion" {
		t.Fatalf("estado tras enviar: %v", o.body)
	}
	is4xx(t, "editar enviada", c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": conPrecio, "cantidad": 1}))
	is4xx(t, "reenviar", c.do("POST", q+"/send", nil))

	// HU-29: respuesta registrada por el dueño; dudas no mueve nada.
	is4xx(t, "canal inválido", c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "paloma"}))
	is4xx(t, "decisión inválida", c.do("POST", q+"/response", map[string]any{"decision": "quizas", "canal": "telefono"}))
	if r = cot(c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "dudas", "canal": "telefono"}))); r.str("estado") != "dudas" {
		t.Fatalf("dudas: %v", r.body)
	}
	if partStock(t, c, "PF-20") != 5 {
		t.Fatal("dudas no debía descontar")
	}

	// HU-33: al aprobar, la orden pasa a En reparación y se descuentan los repuestos en la misma transacción.
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"})))
	if r.str("estado") != "aprobada" || r.str("canalRespuesta") != "mostrador" {
		t.Fatalf("aprobada: %v", r.body)
	}
	if o := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id, nil)); o.str("estado") != "en_reparacion" {
		t.Fatalf("estado tras aprobar: %v", o.body)
	}
	if s := partStock(t, c, "PF-20"); s != 3 {
		t.Fatalf("existencia tras aprobar: %v", s)
	}

	// HU-38: devolver lo que no se usó, ligado a la orden.
	is4xx(t, "devolución sin orden", c.do("POST", "/api/v1/parts/"+pid+"/returns", map[string]any{"cantidad": 1}))
	c.must(http.StatusNoContent, c.do("POST", "/api/v1/parts/"+pid+"/returns", map[string]any{"cantidad": 1, "ordenId": id}))
	if s := partStock(t, c, "PF-20"); s != 4 {
		t.Fatalf("existencia tras devolver: %v", s)
	}
	is4xx(t, "devolver más de lo descontado", c.do("POST", "/api/v1/parts/"+pid+"/returns", map[string]any{"cantidad": 5, "ordenId": id}))
	// R-07: queda 1 unidad por devolver; con 5 devoluciones simultáneas solo una pasa.
	var wg sync.WaitGroup
	var okCount atomic.Int32
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.do("POST", "/api/v1/parts/"+pid+"/returns", map[string]any{"cantidad": 1, "ordenId": id}).status == http.StatusNoContent {
				okCount.Add(1)
			}
		}()
	}
	wg.Wait()
	if okCount.Load() != 1 || partStock(t, c, "PF-20") != 5 {
		t.Fatalf("devoluciones simultáneas: %d pasaron, existencia %v", okCount.Load(), partStock(t, c, "PF-20"))
	}

	// HU-31: cotización adicional durante la reparación.
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "refId": conPrecio, "cantidad": 1})))
	if r.body["numero"] != float64(2) || r.str("estado") != "borrador" {
		t.Fatalf("adicional: %v", r.body)
	}
	// Más unidades de las que hay: se deja cotizar (Q-11) y al aprobar queda pendiente, sin existencia negativa.
	r = cot(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 50})))
	if l := r.body["lineas"].([]any); l[len(l)-1].(map[string]any)["sinStock"] != true {
		t.Fatalf("aviso sin existencia: %v", r.body)
	}
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	if o := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id, nil)); o.str("estado") != "en_reparacion" {
		t.Fatalf("la adicional no cambia el estado: %v", o.body)
	}
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "telefono"}))
	if s := partStock(t, c, "PF-20"); s != 5 {
		t.Fatalf("sin existencia suficiente no debía descontar: %v", s)
	}
}
