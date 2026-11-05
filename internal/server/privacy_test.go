package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// HU-87: el titular pide sus datos o que los borren. El dueño descarga la copia (cliente, autorizaciones, vehículos,
// órdenes y mensajes) y, si pide el borrado, anonimiza: el cliente, sus placas, sus conversaciones y borradores dejan
// de identificarlo y sus órdenes se conservan. La bitácora registra el borrado sin datos del titular. Solo el dueño.
func TestCustomers_ExportAndAnonymize(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550820", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	const phone = "3005550821"
	id := w.customer(phone, "Laura Gómez")
	w.write(phone, "Laura Gómez", "¿cuánto vale la alineación?")
	orderID, _ := w.orderFor(id, "PRV821")

	// Copia de sus datos.
	exp := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/"+id+"/export", nil)).body
	cl := exp["cliente"].(map[string]any)
	if cl["nombre"] != "Laura Gómez" || cl["celular"] != phone || len(exp["autorizaciones"].([]any)) == 0 ||
		len(exp["vehiculos"].([]any)) != 1 || len(exp["ordenes"].([]any)) != 1 || len(exp["mensajes"].([]any)) < 2 {
		t.Fatalf("copia de datos: %v", exp)
	}

	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550822", "nombre": "Yesica", "rol": "recepcion"}))
	rec := e.login("3005550822")
	for _, r := range []resp{rec.do("GET", "/api/v1/customers/"+id+"/export", nil), rec.do("POST", "/api/v1/customers/"+id+"/anonymize", nil)} {
		if r.status != http.StatusForbidden {
			t.Fatalf("recepción atiende solicitudes de datos: %d", r.status)
		}
	}

	// Borrado: anonimiza y conserva la orden.
	done := owner.must(http.StatusOK, owner.do("POST", "/api/v1/customers/"+id+"/anonymize", nil)).body["anonimizados"].(map[string]any)
	if done["vehiculos"] != float64(1) || done["ordenes"] == float64(0) || done["mensajes"] == float64(0) {
		t.Fatalf("anonimizados: %v", done)
	}
	c := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/"+id, nil)).body
	if c["nombre"] != "Cliente anonimizado" || strings.Contains(c["celular"].(string), phone) || c["correo"] != nil {
		t.Fatalf("cliente anonimizado: %v", c)
	}
	if r := owner.do("GET", "/api/v1/vehicles/by-plate/PRV821", nil); r.status != http.StatusNotFound {
		t.Fatalf("la placa real sigue: %d", r.status)
	}
	o := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+orderID, nil)).body
	if !strings.HasPrefix(o["placa"].(string), "ZZ") || o["estado"] == nil {
		t.Fatalf("la orden se conserva sin la placa real: %v", o)
	}
	if r := owner.do("GET", "/api/v1/customers/by-phone/"+phone, nil); r.status != http.StatusNotFound {
		t.Fatalf("el celular ya no lleva al cliente: %d", r.status)
	}
	if list := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers", nil)).body["clientes"].([]any); len(list) != 0 {
		t.Fatalf("el anonimizado sale de la lista: %v", list)
	}

	exp = owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/"+id+"/export", nil)).body
	raw, _ := json.Marshal(exp)
	for _, s := range []string{"Laura", phone, "PRV821", "alineación"} {
		if strings.Contains(string(raw), s) {
			t.Fatalf("la copia después del borrado todavía tiene %q: %s", s, raw)
		}
	}
	if r := owner.do("POST", "/api/v1/customers/"+id+"/anonymize", nil); r.str("code") != "cliente_anonimizado" {
		t.Fatalf("borrar dos veces: %d %v", r.status, r.body)
	}

	for _, ev := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/audit?entidad=cliente", nil)).body["eventos"].([]any) {
		ev := ev.(map[string]any)
		if ev["accion"] != "cliente.anonimizar" {
			continue
		}
		if b, _ := json.Marshal(ev["despues"]); strings.Contains(string(b), "Laura") || strings.Contains(string(b), phone) {
			t.Fatalf("la bitácora del borrado tiene datos del titular: %s", b)
		}
		return
	}
	t.Fatal("el borrado no quedó en la bitácora")
}
