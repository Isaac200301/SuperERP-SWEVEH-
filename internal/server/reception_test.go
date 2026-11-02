package server_test

// HU-13, HU-17, HU-21, HU-22, HU-23 y HU-26 de punta a punta, más la fachada de órdenes que usan
// otros módulos (avisos y agente). Datos Ficticio.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"sweveh/internal/platform/testdb"
	"sweveh/internal/server"
	"sweveh/internal/shared"
)

var jpg = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0}

// upload envía multipart con los campos y, al final, la foto (como lo hace el piso).
func (c *client) upload(path string, fields map[string]string, photo []byte) resp {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if photo != nil {
		fw, _ := mw.CreateFormFile("foto", "foto.jpg")
		_, _ = fw.Write(photo)
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", c.e.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "sweveh")
	req.Header.Set("Origin", c.e.srv.URL)
	res, err := c.http.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out := resp{status: res.StatusCode, header: res.Header, body: map[string]any{}}
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func is4xx(t *testing.T, what string, r resp) {
	t.Helper()
	if r.status/100 != 4 {
		t.Fatalf("%s: esperaba 4xx, obtuve %d %v", what, r.status, r.body)
	}
}

// seedOrder crea cliente, carro con motor y una orden en Por recibir.
func seedOrder(t *testing.T, c *client, phone, plate string) (clienteID, vehiculoID, ordenID string) {
	t.Helper()
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": phone, "nombre": "Cliente " + plate}))
	clienteID = nested(r.body, "cliente", "id").(string)
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{
		"placa": plate, "clienteId": clienteID, "marca": "Renault", "linea": "Logan", "cilindraje": "1600", "turbo": false,
	}))
	vehiculoID = nested(r.body, "vehiculo", "id").(string)
	r = c.must(http.StatusCreated, c.do("POST", "/api/v1/orders", map[string]any{"placa": plate, "sintoma": "Ruido en la suspensión"}))
	return clienteID, vehiculoID, r.str("id")
}

func TestReception_ChecklistEvidenceAssignHistoryCancel(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550130", "Rosa Mejía")
	clienteID, vehiculoID, id := seedOrder(t, c, "3005550130", "ZXC987")
	base := "/api/v1/orders/" + id

	// HU-17: el checklist dice qué falta.
	r := c.must(http.StatusOK, c.do("GET", base+"/checklist", nil))
	if falta, _ := r.body["falta"].([]any); len(falta) != 3 {
		t.Fatalf("falta: %v", r.body)
	}
	is4xx(t, "combustible inválido", c.do("PUT", base+"/checklist", map[string]any{"combustible": "medio"}))
	is4xx(t, "no aplica en carro", c.do("PUT", base+"/checklist", map[string]any{"combustible": "no_aplica"}))
	is4xx(t, "km negativo", c.do("PUT", base+"/checklist", map[string]any{"kilometraje": -1}))
	is4xx(t, "salvedad vacía", c.do("PUT", base+"/checklist", map[string]any{"salvedades": []any{map[string]any{"nota": " "}}}))
	r = c.must(http.StatusOK, c.do("PUT", base+"/checklist", map[string]any{
		"kilometraje": 45000, "combustible": "1/2", "accesorios": []string{"Gato", "gato", " llanta de repuesto "},
		"salvedades": []any{map[string]any{"nota": "Rayón en la puerta"}},
	}))
	if acc, _ := r.body["accesorios"].([]any); len(acc) != 2 {
		t.Fatalf("accesorios sin repetir: %v", r.body)
	}

	// Recibido sin fotos no pasa.
	is4xx(t, "recibido sin fotos", c.do("POST", base+"/transition", map[string]any{"estado": "recibido", "version": 1}))

	// HU-21: fotos de los 4 lados, una nota, y lo que se rechaza.
	for _, lado := range []string{"frente", "trasera", "izquierdo", "derecho"} {
		c.must(http.StatusCreated, c.upload(base+"/evidence", map[string]string{"lado": lado, "mostrarCliente": "true"}, jpg))
	}
	c.must(http.StatusCreated, c.upload(base+"/evidence", map[string]string{"nota": "Buje desgastado"}, nil))
	is4xx(t, "evidencia vacía", c.upload(base+"/evidence", nil, nil))
	is4xx(t, "lado inválido", c.upload(base+"/evidence", map[string]string{"lado": "techo"}, jpg))
	is4xx(t, "no es imagen", c.upload(base+"/evidence", map[string]string{"lado": "frente"}, []byte("hola, no soy una foto")))
	is4xx(t, "sin multipart", c.do("POST", base+"/evidence", map[string]any{"nota": "x"}))

	r = c.must(http.StatusOK, c.do("GET", base+"/evidence", nil))
	ev, _ := r.body["evidencias"].([]any)
	if len(ev) != 5 {
		t.Fatalf("evidencias: %v", r.body)
	}
	var fotoID, notaID string
	for _, x := range ev {
		m := x.(map[string]any)
		if m["tieneFoto"] == true {
			fotoID = m["id"].(string)
		} else {
			notaID = m["id"].(string)
		}
	}
	res, err := c.http.Get(e.srv.URL + base + "/evidence/" + fotoID + "/file")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/jpeg" || !bytes.Equal(body, jpg) {
		t.Fatalf("foto: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	is4xx(t, "nota sin archivo", c.do("GET", base+"/evidence/"+notaID+"/file", nil))
	is4xx(t, "evidencia de otra orden", c.do("GET", base+"/evidence/"+shared.NewID().String()+"/file", nil))

	// Con todo completo pasa a Recibido y el checklist queda cerrado.
	r = c.must(http.StatusOK, c.do("POST", base+"/transition", map[string]any{"estado": "recibido", "version": 1}))
	_ = r
	r = c.must(http.StatusOK, c.do("GET", base+"/checklist", nil))
	if r.body["completadoEn"] == nil {
		t.Fatalf("checklist sin completar: %v", r.body)
	}

	// HU-22 y HU-23: asignar al dueño y verla en "mis órdenes".
	me := c.must(http.StatusOK, c.do("GET", "/api/v1/me", nil))
	userID := nested(me.body, "usuario", "id").(string)
	prometida := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	r = c.must(http.StatusOK, c.do("PATCH", base+"/assignment", map[string]any{"mecanicoId": userID, "fechaPrometida": prometida}))
	if r.str("mecanicoId") != userID {
		t.Fatalf("asignación: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/orders/mine", nil))
	if o, _ := r.body["ordenes"].([]any); len(o) != 1 {
		t.Fatalf("mis órdenes: %v", r.body)
	}

	// Fachada que usan avisos y agente, dentro de la transacción del taller.
	tid := tallerOf(t, c)
	m := server.Build(e.db.Tx, e.st, e.clk, e.log)
	oid, _ := shared.ParseID(id)
	vid, _ := shared.ParseID(vehiculoID)
	cid, _ := shared.ParseID(clienteID)
	err = e.db.Tx.Do(testdb.As(tid), func(ctx context.Context) error {
		if s, err := m.OrdersF.Load(ctx, tid, oid); err != nil || s.State != "recibido" {
			t.Fatalf("load: %+v %v", s, err)
		}
		if err := m.OrdersF.Move(ctx, tid, oid, "en_diagnostico", shared.Actor{Type: shared.ActorAgent}, "diagnóstico"); err != nil {
			t.Fatalf("move: %v", err)
		}
		if l, err := m.OrdersF.OpenByVehicle(ctx, tid, vid); err != nil || len(l) != 1 {
			t.Fatalf("abiertas del vehículo: %v %v", l, err)
		}
		if i, err := m.OrdersF.Info(ctx, tid, oid); err != nil || i.Vehicle != "Renault Logan" {
			t.Fatalf("info: %+v %v", i, err)
		}
		if l, err := m.OrdersF.InState(ctx, tid, "en_diagnostico"); err != nil || len(l) != 1 {
			t.Fatalf("en estado: %v %v", l, err)
		}
		if l, err := m.OrdersF.OpenByCustomer(ctx, tid, cid, []shared.ID{vid}); err != nil || len(l) != 1 {
			t.Fatalf("abiertas del cliente: %v %v", l, err)
		}
		if p, err := m.OrdersF.LastShownPhoto(ctx, tid, oid); err != nil || p == nil {
			t.Fatalf("última foto para el cliente: %v %v", p, err)
		}
		_ = m.OrdersF.Stats()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// HU-13: el historial del vehículo trae la orden y sus evidencias.
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/vehicles/"+vehiculoID+"/history", nil))
	h, _ := r.body["ordenes"].([]any)
	if len(h) != 1 || len(nested(h[0].(map[string]any), "evidencias").([]any)) != 5 {
		t.Fatalf("historial: %v", r.body)
	}

	// HU-26: cancelar exige motivo; cerrada ya no se mueve.
	v := r2v(c, id)
	is4xx(t, "cancelar sin motivo", c.do("POST", base+"/cancel", map[string]any{"motivo": " ", "version": v}))
	r = c.must(http.StatusOK, c.do("POST", base+"/cancel", map[string]any{"motivo": "El cliente no volvió", "version": v}))
	if r.str("estado") != "cancelada" {
		t.Fatalf("cancelada: %v", r.body)
	}
	is4xx(t, "checklist en orden cerrada", c.do("PUT", base+"/checklist", map[string]any{"kilometraje": 1}))
}
