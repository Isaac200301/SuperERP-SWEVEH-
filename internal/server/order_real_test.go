package server_test

// Lo que Pablo encontró probando la orden en el navegador (3-nov-2026), de punta a punta: varios ítems con
// cantidad, devolución que se ve, varias fotos por nota y en los hallazgos, mano de obra en el diagnóstico.
// Datos Ficticio.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

// uploadPhotos envía una nota con varias fotos en un solo formulario, como lo hace la pantalla.
func (c *client) uploadPhotos(path string, fields map[string]string, n int) resp {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for i := 0; i < n; i++ {
		fw, _ := mw.CreateFormFile("foto", "foto.jpg")
		_, _ = fw.Write(jpg)
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "sweveh")
	req.Header.Set("Origin", c.e.srv.URL)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	out := resp{status: res.StatusCode, header: res.Header, body: map[string]any{}}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out.body)
	return out
}

// orderInDiagnosis deja una orden en diagnóstico con un repuesto de existencia conocida.
func orderInDiagnosis(t *testing.T, c *client, plate string, stock int) (orderID, partID string) {
	t.Helper()
	partID = c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "PF-" + plate, "nombre": "Pastillas", "tipoVehiculo": "carro", "costo": 40000, "precioVenta": 65000, "existencia": stock, "minimo": 0,
	})).str("id")
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550" + plate[3:], "nombre": "Cliente " + plate}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": plate, "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "1600", "energia": "gasolina", "aspiracion": "atmosferico", "transmision": "manual"}))
	orderID = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": plate, "sintoma": "Frena mal", "estadoInicial": "recibido"})).str("id")
	c.must(http.StatusOK, c.do("POST", "/api/v1/orders/"+orderID+"/transition", map[string]any{"estado": "en_diagnostico", "version": r2v(c, orderID)}))
	return orderID, partID
}

func lines(r resp) []map[string]any {
	var out []map[string]any
	for _, l := range cot(r).body["lineas"].([]any) {
		out = append(out, l.(map[string]any))
	}
	return out
}

func TestRealOrder_QuoteQuantitiesAndVisibleReturn(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550300", "Pablo Rodríguez")
	id, pid := orderInDiagnosis(t, c, "RQT301", 5)
	q := "/api/v1/orders/" + id + "/quote"

	// El mismo repuesto dos veces: una línea con cantidad 2, no dos líneas de 1.
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 1}))
	ls := lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 1})))
	if len(ls) != 1 || ls[0]["cantidad"] != float64(2) {
		t.Fatalf("repetir repuesto debía sumar cantidad: %v", ls)
	}
	// Cambiar la cantidad de una línea; el precio de lista no se cambia desde la cotización.
	line := ls[0]["id"].(string)
	r := cot(c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+line, map[string]any{"cantidad": 3})))
	if r.body["total"] != float64(3*65000) {
		t.Fatalf("cantidad 3: %v", r.body)
	}
	if r := c.do("PATCH", q+"/lines/"+line, map[string]any{"precioUnitario": 1}); r.status < 400 {
		t.Fatalf("no se debía poder cambiar el precio de lista de un repuesto: %d %v", r.status, r.body)
	}
	is4xx(t, "cantidad cero", c.do("PATCH", q+"/lines/"+line, map[string]any{"cantidad": 0}))
	// Mano de obra libre: el precio lo pone la persona y sí se puede corregir.
	ls = lines(c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra frenos", "cantidad": 1, "precioUnitario": 60000})))
	labor := ls[1]["id"].(string)
	if r := cot(c.must(http.StatusOK, c.do("PATCH", q+"/lines/"+labor, map[string]any{"precioUnitario": 70000}))); r.body["total"] != float64(3*65000+70000) {
		t.Fatalf("corregir mano de obra: %v", r.body)
	}

	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))

	// La devolución dice qué pasó y la orden muestra descontado y devuelto por repuesto.
	r = c.must(http.StatusOK, c.do("POST", "/api/v1/parts/"+pid+"/returns", map[string]any{"cantidad": 1, "ordenId": id}))
	if r.body["devueltas"] != float64(1) || r.body["quedanPorDevolver"] != float64(2) || r.body["existencia"] != float64(3) {
		t.Fatalf("respuesta de la devolución: %d %v", r.status, r.body)
	}
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id+"/part-movements", nil))
	m := r.body["repuestos"].([]any)[0].(map[string]any)
	if m["repuestoId"] != pid || m["descontado"] != float64(3) || m["devuelto"] != float64(1) || m["pendiente"] != float64(0) {
		t.Fatalf("movimientos de la orden: %v", r.body)
	}
}

func TestRealOrder_PendingPartHasNothingToReturn(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550310", "Pablo Rodríguez")
	id, pid := orderInDiagnosis(t, c, "RQT311", 0)
	q := "/api/v1/orders/" + id + "/quote"
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": pid, "cantidad": 2}))
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	r := c.must(http.StatusOK, c.do("GET", "/api/v1/orders/"+id+"/part-movements", nil))
	m := r.body["repuestos"].([]any)[0].(map[string]any)
	if m["descontado"] != float64(0) || m["pendiente"] != float64(2) {
		t.Fatalf("sin existencia queda pendiente y no se descuenta: %v", r.body)
	}
	if r := c.do("POST", "/api/v1/parts/"+pid+"/returns", map[string]any{"cantidad": 1, "ordenId": id}); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("no hay nada que devolver: %d %v", r.status, r.body)
	}
}

func TestRealOrder_PhotosPerNoteAndFindingWithLabor(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550320", "Pablo Rodríguez")
	id, _ := orderInDiagnosis(t, c, "RQT321", 1)
	base := "/api/v1/orders/" + id

	// Una nota con 3 fotos: 3 evidencias del mismo grupo, la nota en la primera.
	r := c.must(http.StatusCreated, c.uploadPhotos(base+"/evidence", map[string]string{"nota": "Rayón en la puerta trasera"}, 3))
	grupo := r.str("grupoId")
	if grupo == "" {
		t.Fatalf("la nota con fotos debía devolver su grupo: %v", r.body)
	}
	ev := c.must(http.StatusOK, c.do("GET", base+"/evidence", nil)).body["evidencias"].([]any)
	n := 0
	for _, x := range ev {
		if x.(map[string]any)["grupoId"] == grupo {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("debían quedar 3 fotos en el grupo: %d de %v", n, ev)
	}
	is4xx(t, "más de 8 fotos", c.uploadPhotos(base+"/evidence", map[string]string{"nota": "x"}, 9))

	// Hallazgo con mano de obra estimada y sus fotos.
	r = c.must(http.StatusCreated, c.do("POST", base+"/findings", map[string]any{"sistema": "carroceria", "descripcion": "Golpe en el bómper",
		"gravedad": "recomendado", "manoObra": 120000}))
	fid := r.str("id")
	if r.body["manoObra"] != float64(120000) {
		t.Fatalf("mano de obra del hallazgo: %v", r.body)
	}
	is4xx(t, "mano de obra negativa", c.do("POST", base+"/findings", map[string]any{"sistema": "motor", "descripcion": "x", "gravedad": "urgente", "manoObra": -1}))
	c.must(http.StatusCreated, c.uploadPhotos(base+"/evidence", map[string]string{"hallazgoId": fid}, 2))
	fotos := 0
	for _, x := range c.must(http.StatusOK, c.do("GET", base+"/evidence", nil)).body["evidencias"].([]any) {
		if x.(map[string]any)["hallazgoId"] == fid {
			fotos++
		}
	}
	if fotos != 2 {
		t.Fatalf("fotos del hallazgo: %d", fotos)
	}
	// Un hallazgo de otra orden no se puede usar.
	otra, _ := orderInDiagnosis(t, c, "RQT322", 1)
	is4xx(t, "hallazgo de otra orden", c.uploadPhotos("/api/v1/orders/"+otra+"/evidence", map[string]string{"hallazgoId": fid}, 1))
}
