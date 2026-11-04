package server_test

// Diagnóstico en la orden (HU-103 propuesta) de punta a punta: estados en que se permite, validación,
// el mecánico registra, borrar solo antes de cotizar, bitácora y aislamiento entre talleres. Datos Ficticio.

import (
	"context"
	"net/http"
	"testing"

	"sweveh/internal/platform/testdb"
)

func TestFindings_DiagnoseByMechanicWithIsolation(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550180", "Lucía Toro")
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/users", map[string]any{"celular": "3005550181", "nombre": "Pedro Mecánico", "rol": "mecanico"}))
	hallazgo := map[string]any{"sistema": "frenos", "descripcion": "Pastillas delanteras al 10 %", "gravedad": "urgente", "codigosFalla": []string{"c1234"}}

	// Por recibir: el vehículo aún no está en el taller.
	_, _, pendiente := seedOrder(t, owner, "3005550182", "DGN123")
	r := owner.do("POST", "/api/v1/orders/"+pendiente+"/findings", hallazgo)
	if r.status != http.StatusUnprocessableEntity || r.str("code") != "hallazgo_fuera_de_estado" {
		t.Fatalf("hallazgo en Por recibir: %d %v", r.status, r.body)
	}

	// Orden recibida: el mecánico registra el diagnóstico.
	r = owner.must(http.StatusCreated, owner.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550183", "nombre": "Sara Gil"}))
	owner.must(http.StatusCreated, owner.do("POST", "/api/v1/vehicles", map[string]any{"placa": "DGN456", "clienteId": nested(r.body, "cliente", "id"),
		"cilindraje": "1600", "energia": "gasolina", "aspiracion": "atmosferico", "transmision": "manual"}))
	r = owner.must(http.StatusCreated, owner.do("POST", "/api/v1/orders", map[string]any{"placa": "DGN456", "sintoma": "Chilla al frenar", "estadoInicial": "recibido"}))
	id := r.str("id")
	base := "/api/v1/orders/" + id + "/findings"

	mec := e.login("3005550181")
	for campo, malo := range map[string]any{"sistema": "radio", "gravedad": "grave", "codigosFalla": []string{"X9999"}} {
		in := map[string]any{}
		for k, v := range hallazgo {
			in[k] = v
		}
		in[campo] = malo
		is4xx(t, "hallazgo con "+campo+" inválido", mec.do("POST", base, in))
	}
	r = mec.must(http.StatusCreated, mec.do("POST", base, hallazgo))
	fid := r.str("id")
	if codes := r.body["codigosFalla"].([]any); len(codes) != 1 || codes[0] != "C1234" {
		t.Fatalf("códigos normalizados: %v", r.body)
	}
	mec.must(http.StatusCreated, mec.do("POST", base, map[string]any{"sistema": "suspension", "descripcion": "Bujes gastados", "gravedad": "recomendado"}))
	r = owner.must(http.StatusOK, owner.do("GET", base, nil))
	if l := r.body["hallazgos"].([]any); len(l) != 2 {
		t.Fatalf("lista: %v", r.body)
	}

	// Otro taller no ve ni borra los hallazgos (RLS + taller de la sesión).
	other := e.register("3105550189", "Otro Taller")
	if r := other.do("GET", base, nil); r.status != http.StatusNotFound {
		t.Fatalf("otro taller ve hallazgos: %d %v", r.status, r.body)
	}
	if r := other.do("DELETE", base+"/"+fid, nil); r.status != http.StatusNotFound {
		t.Fatalf("otro taller borra hallazgos: %d %v", r.status, r.body)
	}

	// Antes de cotizar se puede borrar; queda en la bitácora.
	mec.must(http.StatusNoContent, mec.do("DELETE", base+"/"+fid, nil))
	var n int
	err := e.db.Tx.Do(testdb.As(tallerOf(t, owner)), func(ctx context.Context) error {
		return queryRow(ctx, `SELECT count(*) FROM audit.evento WHERE accion LIKE 'orden.hallazgo.%'`, &n)
	})
	if err != nil || n != 3 {
		t.Fatalf("bitácora de hallazgos: %d %v", n, err)
	}
	if r := mec.do("DELETE", base+"/"+fid, nil); r.status != http.StatusNotFound {
		t.Fatalf("borrar dos veces: %d %v", r.status, r.body)
	}
}
