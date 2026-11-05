package server_test

import (
	"net/http"
	"testing"
)

// HU-15: el sistema avisa de posibles clientes repetidos (mismo nombre con otro celular), los muestra juntos y, al
// unirlos, todos los vehículos y órdenes quedan en el que se conserva. Si el repetido vuelve a escribir desde su
// celular, se le reconoce como el que se conservó. La unión queda en la bitácora con quién la hizo y qué se unió.
// Solo el dueño une.
func TestCustomers_MergeDuplicates(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550800", "Rosa Pérez")
	customer := func(phone, name, plate string) string {
		r := owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"celular": phone, "nombre": name}))
		id := nested(r.body, "cliente", "id").(string)
		owner.must(http.StatusCreated, owner.do("POST", "/api/v1/vehicles", map[string]any{"placa": plate, "clienteId": id}))
		return id
	}
	keep := customer("3005550801", "Carlos Ruiz", "UNI801")
	drop := customer("3005550802", "  carlos   RUIZ ", "UNI802")
	customer("3005550803", "Ana Ríos", "UNI803")
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/orders", map[string]any{"placa": "UNI802", "sintoma": "Frenos"}))

	groups := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/duplicates", nil)).body["grupos"].([]any)
	if len(groups) != 1 || len(groups[0].(map[string]any)["clientes"].([]any)) != 2 {
		t.Fatalf("repetidos: %v", groups)
	}

	if r := owner.do("POST", "/api/v1/customers/merge", map[string]any{"conservar": keep, "unir": keep}); r.status != http.StatusBadRequest {
		t.Fatalf("unir consigo mismo: %d", r.status)
	}
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550804", "nombre": "Yesica", "rol": "recepcion"}))
	if r := e.login("3005550804").do("POST", "/api/v1/customers/merge", map[string]any{"conservar": keep, "unir": drop}); r.status != http.StatusForbidden {
		t.Fatalf("recepción une clientes: %d", r.status)
	}

	r := owner.must(http.StatusOK, owner.do("POST", "/api/v1/customers/merge", map[string]any{"conservar": keep, "unir": drop}))
	moved := r.body["movidos"].(map[string]any)
	if nested(r.body, "cliente", "id") != keep || moved["vehiculos"] != float64(1) || moved["ordenes"] != float64(1) {
		t.Fatalf("unir: %v", r.body)
	}
	if vs := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/"+keep+"/vehicles", nil)).body["vehiculos"].([]any); len(vs) != 2 {
		t.Fatalf("vehículos del que se conserva: %v", vs)
	}
	orders := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders", nil)).body["ordenes"].([]any)
	if len(orders) != 1 || orders[0].(map[string]any)["cliente"].(map[string]any)["id"] != keep {
		t.Fatalf("la orden quedó con el que se conserva: %v", orders)
	}
	if list := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers", nil)).body["clientes"].([]any); len(list) != 2 {
		t.Fatalf("el repetido sale de la lista: %d", len(list))
	}
	if g := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/duplicates", nil)).body["grupos"].([]any); len(g) != 0 {
		t.Fatalf("ya no hay repetidos: %v", g)
	}
	if c := owner.must(http.StatusOK, owner.do("GET", "/api/v1/customers/by-phone/3005550802", nil)).body; c["id"] != keep {
		t.Fatalf("el celular del repetido lleva al que se conserva: %v", c)
	}
	if r := owner.do("POST", "/api/v1/customers/merge", map[string]any{"conservar": keep, "unir": drop}); r.str("code") != "cliente_ya_unido" {
		t.Fatalf("unir dos veces: %d %v", r.status, r.body)
	}

	var found bool
	for _, ev := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/audit?entidad=cliente", nil)).body["eventos"].([]any) {
		ev := ev.(map[string]any)
		if ev["accion"] != "cliente.unir" {
			continue
		}
		found = true
		before := ev["antes"].(map[string]any)["unido"].(map[string]any)
		if ev["entidadId"] != keep || ev["actorId"] == nil || before["id"] != drop || before["celular"] != "+573005550802" {
			t.Fatalf("bitácora de la unión: %v", ev)
		}
	}
	if !found {
		t.Fatal("la unión no quedó en la bitácora")
	}
}
