package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// Pruebas del T4 (números y catálogo) contra la API real y Postgres, criterio por criterio de docs/04.

// kpi devuelve el KPI con ese código del tablero.
func kpi(t *testing.T, body map[string]any, code string) map[string]any {
	t.Helper()
	for _, k := range body["kpis"].([]any) {
		if m := k.(map[string]any); m["codigo"] == code {
			return m
		}
	}
	t.Fatalf("no hay KPI %q: %v", code, body["kpis"])
	return nil
}

// motoOrder abre una orden "Recibido" para una moto nueva con su cliente.
func motoOrder(t *testing.T, c *client, plate string) string {
	t.Helper()
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005551" + plate[3:5] + "0", "nombre": "Moto " + plate}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": plate, "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "150", "energia": "gasolina", "ciclo": "4t", "refrigeracion": "aire", "transmision": "manual"}))
	return c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": plate, "sintoma": "Cambio de aceite",
		"estadoInicial": "recibido"})).str("id")
}

// HU-71 (criterio 2) y HU-68: las órdenes de ejemplo suman para que el tablero no salga en cero y sus números van
// marcados "ejemplo" mientras existan (docs/03, taller ficticio). El conteo por estado coincide con uno manual.
func TestDashboard_ExampleMarkAndStateCounts(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550600", "Rosa Pérez")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550601", "nombre": "Cliente Ejemplo"}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "EJE601", "clienteId": nested(r.body, "cliente", "id")}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "EJE601", "sintoma": "Ejemplo", "ejemplo": true}))

	d := c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil)).body
	if d["ejemplo"] != true || kpi(t, d, "abiertas")["ejemplo"] != true || kpi(t, d, "abiertas")["texto"] != "1" {
		t.Fatalf("solo ejemplo: %v", d)
	}

	orderInDiagnosis(t, c, "REA602", 1)
	motoOrder(t, c, "MTO60F")
	d = c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil)).body
	byState := d["porEstado"].(map[string]any)
	if d["ejemplo"] != true || kpi(t, d, "abiertas")["ejemplo"] != true || kpi(t, d, "abiertas")["texto"] != "3" ||
		byState["en_diagnostico"] != float64(1) || byState["recibido"] != float64(1) || byState["por_recibir"] != float64(1) {
		t.Fatalf("órdenes propias y de ejemplo: %v", d)
	}
	// HU-68 (Should): por tipo, carros y motos por separado.
	car := c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard?tipo=carro", nil)).body
	moto := c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard?tipo=moto", nil)).body
	if car["tipo"] != "carro" || kpi(t, car, "abiertas")["texto"] != "2" || car["porEstado"].(map[string]any)["en_diagnostico"] != float64(1) ||
		kpi(t, moto, "abiertas")["texto"] != "1" || moto["porEstado"].(map[string]any)["recibido"] != float64(1) {
		t.Fatalf("por tipo: carro %v, moto %v", car, moto)
	}
	// Sin órdenes de ejemplo, sin marca.
	e2 := newEnv(t)
	c2 := e2.register("3105550609", "Iván Soto")
	orderInDiagnosis(t, c2, "SIN609", 1)
	if d := c2.must(http.StatusOK, c2.do("GET", "/api/v1/dashboard", nil)).body; d["ejemplo"] != false || kpi(t, d, "abiertas")["ejemplo"] != false {
		t.Fatalf("sin ejemplo: %v", d)
	}
}

// HU-69, HU-70 y HU-71: una aprobación pendiente se ve "1 · 26 h"; el tiempo promedio sale en horas y días hábiles; el
// ticket va en pesos sin decimales; y el filtro por tipo separa el promedio de carros del de motos.
func TestDashboard_ApprovalsCycleAndTicket(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550610", "Rosa Pérez")
	id, _ := orderInDiagnosis(t, c, "TIC611", 1)
	base := "/api/v1/orders/" + id
	tr := func(body map[string]any) {
		body["version"] = r2v(c, id)
		c.must(http.StatusOK, c.do("POST", base+"/transition", body))
	}
	c.must(http.StatusOK, c.do("POST", base+"/quote/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra", "cantidad": 1,
		"precioUnitario": 100000}))
	c.must(http.StatusOK, c.do("POST", base+"/quote/send", nil))
	c.must(http.StatusOK, c.do("POST", base+"/quote/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	e.clk.Advance(30 * time.Hour)
	c = e.login("3105550610") // pasaron más de 24 h: la sesión venció
	tr(map[string]any{"estado": "listo"})
	tr(map[string]any{"estado": "entregado", "recogidoPor": "Laura Gómez", "accesoriosConfirmados": true})

	pend, _ := orderInDiagnosis(t, c, "PEN612", 1)
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+pend+"/quote/lines", map[string]any{"tipo": "servicio", "descripcion": "Revisión",
		"cantidad": 1, "precioUnitario": 50000}))
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+pend+"/quote/send", nil))
	e.clk.Advance(26*time.Hour + 10*time.Minute)
	c = e.login("3105550610") // pasaron más de 24 h: la sesión venció

	d := c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil)).body
	if k := kpi(t, d, "aprobaciones"); k["texto"] != "1 · 26 h" {
		t.Fatalf("HU-69: %v", k)
	}
	k := kpi(t, d, "tiempo")
	if v, _ := k["valor"].(float64); v < 29.9 || v > 30.1 || !strings.Contains(k["texto"].(string), "30 h · ") ||
		!strings.Contains(k["texto"].(string), "días hábiles") {
		t.Fatalf("HU-70: %v", k)
	}
	// Total aprobado de 100.000 (el taller nuevo tiene IVA 0 %): en pesos y sin decimales.
	if k := kpi(t, d, "ticket"); k["texto"] != "$100.000" || k["valor"] != float64(100000) {
		t.Fatalf("HU-71: %v", k)
	}
	if k := kpi(t, c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard?tipo=moto", nil)).body, "tiempo"); k["texto"] != "sin datos" {
		t.Fatalf("HU-70 por tipo, motos sin entregas: %v", k)
	}
}

// HU-73: la mediana del mes cuenta lo que tarda el taller en contestar; las respuestas automáticas (estado AU-05 y
// autorización de datos) salen al instante y se cuentan aparte para no maquillar el número.
// HU-72: un vehículo con dos visitas en 12 meses cuenta como vehículo que volvió; las órdenes canceladas no cuentan.
func TestDashboard_ReturningVehicles(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550620", "Rosa Pérez")
	deliver := func(id string) {
		base := "/api/v1/orders/" + id
		c.must(http.StatusOK, c.do("POST", base+"/quote/lines", map[string]any{"tipo": "servicio", "descripcion": "Revisión", "cantidad": 1,
			"precioUnitario": 50000}))
		c.must(http.StatusOK, c.do("POST", base+"/quote/send", nil))
		c.must(http.StatusOK, c.do("POST", base+"/quote/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
		c.must(http.StatusOK, c.do("POST", base+"/transition", map[string]any{"estado": "listo", "version": r2v(c, id)}))
		c.must(http.StatusOK, c.do("POST", base+"/transition", map[string]any{"estado": "entregado", "recogidoPor": "Carlos",
			"accesoriosConfirmados": true, "version": r2v(c, id)}))
	}
	// Carlos: dos visitas. Laura: una visita y otra orden cancelada.
	first, _ := orderInDiagnosis(t, c, "CAR621", 1)
	deliver(first)
	c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "CAR621", "sintoma": "Cambio de aceite", "estadoInicial": "recibido"}))
	once, _ := orderInDiagnosis(t, c, "LAU622", 1)
	deliver(once)
	cancelled := c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": "LAU622", "sintoma": "Ruido"})).str("id")
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+cancelled+"/cancel", map[string]any{"motivo": "No vino", "version": r2v(c, cancelled)}))

	k := kpi(t, c.must(http.StatusOK, c.do("GET", "/api/v1/dashboard", nil)).body, "vuelven")
	if k["texto"] != "50 %" || k["detalle"] != "1 de 2 vehículos" {
		t.Fatalf("clientes que vuelven: %v", k)
	}
}

func TestDashboard_ResponseMedianSeparatesAutomatic(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550620", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const andres = "3001234620"
	cid := w.customer(andres, "Andrés Ruiz") // "Hola" → autorización (automática)
	w.orderFor(cid, "RES621")
	w.write(andres, "Andrés Ruiz", "ya está mi carro?") // AU-05 al instante (automática)
	w.write(andres, "Andrés Ruiz", "y me pueden dar factura electrónica?")
	conv, _ := w.conversation(andres)
	e.clk.Advance(12 * time.Minute)
	owner = e.login("3105550620") // el reloj de prueba avanzó más que la vida de la sesión
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/conversations/"+conv["id"].(string)+"/messages", map[string]any{"texto": "Claro que sí."}))

	k := kpi(t, owner.must(http.StatusOK, owner.do("GET", "/api/v1/dashboard", nil)).body, "respuesta")
	if k["texto"] != "12 min" || !strings.Contains(k["detalle"].(string), "respuestas automáticas") {
		t.Fatalf("HU-73: %v", k)
	}
}

// HU-40: el catálogo trae 9 tarjetas (5 encendidas, 3 apagadas y AU-09 "próximamente") con el disparador en palabras del
// taller. HU-42: {vehiculo} dice "tu moto" o "tu carro"; una variable mal escrita no se guarda; los avisos que salen
// fuera de la ventana de 24 h avisan que en producción cambiar el texto pide aprobación de Meta.
func TestAutomations_CatalogAndTexts(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550630", "Rosa Pérez")
	cards := owner.must(http.StatusOK, owner.do("GET", "/api/v1/automations", nil)).body["automatizaciones"].([]any)
	on, off, soon := 0, 0, 0
	byCode := map[string]map[string]any{}
	for _, x := range cards {
		c := x.(map[string]any)
		byCode[c["codigo"].(string)] = c
		switch {
		case c["proximamente"] == true:
			soon++
		case c["activa"] == true:
			on++
		default:
			off++
		}
		if strings.TrimSpace(c["disparador"].(string)) == "" {
			t.Errorf("%s sin disparador", c["codigo"])
		}
	}
	if len(cards) != 9 || on != 5 || off != 3 || soon != 1 || byCode["AU-09"]["proximamente"] != true ||
		byCode["AU-03"]["disparador"] != "La orden pasa a Listo" {
		t.Fatalf("HU-40: %d tarjetas, %d encendidas, %d apagadas, %d próximamente", len(cards), on, off, soon)
	}
	if byCode["AU-06"]["fueraDeVentana"] != true || byCode["AU-07"]["fueraDeVentana"] != true || byCode["AU-01"]["fueraDeVentana"] != false {
		t.Fatalf("HU-42 criterio 3: %v %v", byCode["AU-06"], byCode["AU-01"])
	}
	if r := owner.do("PATCH", "/api/v1/automations/AU-01", map[string]any{"texto": "Hola {clinete}, recibimos tu {vehiculo}."}); r.status != http.StatusBadRequest ||
		r.str("code") != "dato_desconocido" {
		t.Fatalf("HU-42 criterio 2: %d %v", r.status, r.body)
	}
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/automations/AU-01", map[string]any{"texto": "Hola {cliente}, recibimos tu {vehiculo} ({modelo}) {placa}."}))

	// AU-01 al recibir: "tu carro" para el carro y "tu moto" para la moto.
	w := e.whatsapp(t, owner, nil)
	const ana, beto = "3001234631", "3001234632"
	carID, _ := w.orderFor(w.customer(ana, "Ana Ruiz"), "CAR631")
	w.move(carID, "recibido", nil)
	if got := w.sentBy(ana, "AU-01"); len(got) != 1 || !strings.Contains(got[0]["cuerpo"].(string), "recibimos tu carro (Renault Logan) CAR631") {
		t.Fatalf("HU-42 carro: %v", got)
	}
	bcid := w.customer(beto, "Beto Gil")
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/vehicles", map[string]any{"placa": "MTO63F", "clienteId": bcid,
		"cilindraje": "150", "energia": "gasolina", "ciclo": "4t", "refrigeracion": "aire", "transmision": "manual"}))
	mid := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/orders", map[string]any{"placa": "MTO63F", "sintoma": "Aceite"})).str("id")
	owner.must(http.StatusOK, owner.do("PUT", "/api/v1/orders/"+mid+"/checklist", map[string]any{"combustible": "1/2"}))
	w.move(mid, "recibido", nil)
	if got := w.sentBy(beto, "AU-01"); len(got) != 1 || !strings.Contains(got[0]["cuerpo"].(string), "recibimos tu moto") {
		t.Fatalf("HU-42 moto: %v", got)
	}
}

// HU-22: al preguntar por el vehículo, con fecha prometida la respuesta la dice; sin fecha, da el estado y "te avisamos
// apenas esté listo", sin inventar una.
func TestAgent_StatusWithAndWithoutPromise(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550650", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const andres = "3001234650"
	id, _ := w.orderFor(w.customer(andres, "Andrés Ruiz"), "PRO651")

	w.write(andres, "Andrés Ruiz", "ya está mi carro?")
	if got := w.last(andres)["cuerpo"].(string); !strings.Contains(got, "Te avisamos apenas esté listo") || strings.Contains(got, "prometido") {
		t.Fatalf("sin fecha prometida: %q", got)
	}
	when := e.clk.Now().Add(6 * time.Hour)
	owner.must(http.StatusOK, owner.do("PATCH", "/api/v1/orders/"+id+"/assignment", map[string]any{"fechaPrometida": when.Format(time.RFC3339)}))
	w.write(andres, "Andrés Ruiz", "cómo va mi carro?")
	if got := w.last(andres)["cuerpo"].(string); !strings.Contains(got, "Te lo tenemos prometido para") || strings.Contains(got, "Te avisamos apenas") {
		t.Fatalf("con fecha prometida: %q", got)
	}
}
