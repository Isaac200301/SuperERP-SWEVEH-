package server_test

import (
	"net/http"
	"testing"
)

// HU-26: cancelar pide el motivo y la orden sale de las abiertas. Si ya se había aprobado, los repuestos descontados
// vuelven a bodega con su movimiento de devolución. En el historial del vehículo la orden sale cancelada con su motivo.
func TestOrders_CancelReturnsPartsAndShowsInHistory(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550840", "Rosa Pérez")
	w := e.whatsapp(t, owner, nil)
	cid := w.customer("3001234840", "Laura Gómez")
	id, part := w.orderFor(cid, "CAN840")
	w.move(id, "recibido", nil)
	w.move(id, "en_diagnostico", nil)
	q := "/api/v1/orders/" + id + "/quote"
	owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "repuesto", "refId": part, "cantidad": 2}))
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))
	owner.must(http.StatusOK, owner.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	if s := partStock(t, owner, "FA-CAN840"); s != 3 {
		t.Fatalf("existencia tras aprobar: %v", s)
	}

	cancel := "/api/v1/orders/" + id + "/cancel"
	if r := owner.do("POST", cancel, map[string]any{"motivo": " ", "version": r2v(owner, id)}); r.status < 400 {
		t.Fatalf("cancelar sin motivo: %d", r.status)
	}
	owner.must(http.StatusOK, owner.do("POST", cancel, map[string]any{"motivo": "El cliente no volvió", "version": r2v(owner, id)}))
	if s := partStock(t, owner, "FA-CAN840"); s != 5 {
		t.Fatalf("los repuestos no volvieron: existencia %v", s)
	}
	movs := owner.must(http.StatusOK, owner.do("GET", "/api/v1/parts/"+part+"/movements", nil)).body["movimientos"].([]any)
	var back bool
	for _, m := range movs {
		m := m.(map[string]any)
		if m["motivo"] == "devolucion" && m["cantidad"] == float64(2) && m["ordenId"] == id {
			back = true
		}
	}
	if !back {
		t.Fatalf("movimiento de devolución: %v", movs)
	}
	for _, o := range owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders?abiertas=1", nil)).body["ordenes"].([]any) {
		if o.(map[string]any)["id"] == id {
			t.Fatal("la orden cancelada sigue abierta")
		}
	}
	v := owner.must(http.StatusOK, owner.do("GET", "/api/v1/vehicles/by-plate/CAN840", nil)).body
	hist := owner.must(http.StatusOK, owner.do("GET", "/api/v1/vehicles/"+v["id"].(string)+"/history", nil)).body
	var found bool
	for _, it := range hist["ordenes"].([]any) {
		o := it.(map[string]any)["orden"].(map[string]any)
		if o["id"] == id && o["estado"] == "cancelada" && o["motivoCancelacion"] == "El cliente no volvió" {
			found = true
		}
	}
	if !found {
		t.Fatalf("historial del vehículo: %v", hist)
	}
}
