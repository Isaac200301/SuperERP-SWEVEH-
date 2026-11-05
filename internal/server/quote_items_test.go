package server_test

import (
	"net/http"
	"strings"
	"testing"
)

// HU-30: con una cotización de 3 ítems el cliente pulsa "Elegir ítems", escribe "1 y 3" y la orden sigue solo con esos
// 2: el total se recalcula y se le confirma. Un número que no está en la cotización no registra nada y se le pide de
// nuevo. Con un solo ítem no aparece "Elegir ítems".
func TestWhatsApp_ApproveQuoteItemByItem(t *testing.T) {
	e := newEnv(t)
	owner := e.register("3105550830", "Pablo Rodríguez")
	w := e.whatsapp(t, owner, nil)
	const phone = "3001234830"
	cid := w.customer(phone, "Laura Gómez")
	id, _ := w.orderFor(cid, "ITM830")
	w.move(id, "recibido", nil)
	w.move(id, "en_diagnostico", nil)
	q := "/api/v1/orders/" + id + "/quote"
	for _, l := range []map[string]any{
		{"descripcion": "Cambio de pastillas", "precio": 100000},
		{"descripcion": "Rectificar discos", "precio": 80000},
		{"descripcion": "Lavado de motor", "precio": 20000},
	} {
		owner.must(http.StatusOK, owner.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": l["descripcion"], "cantidad": 1,
			"precioUnitario": l["precio"]}))
	}
	owner.must(http.StatusOK, owner.do("POST", q+"/send", nil))
	w.pump()

	au2 := w.sentBy(phone, "AU-02")
	body := au2[0]["cuerpo"].(string)
	if len(au2) != 1 || !strings.Contains(body, "1. Cambio de pastillas") || !strings.Contains(body, "3. Lavado de motor") ||
		!hasPrefix(buttons(au2[0]), "quote:elegir:"+id) || len(buttons(au2[0])) != 3 {
		t.Fatalf("AU-02 con ítems numerados y Elegir ítems: %v", au2)
	}
	w.press(phone, "Laura Gómez", "quote:elegir:"+id, "Elegir ítems")
	if !strings.Contains(w.last(phone)["cuerpo"].(string), "Escribe los números") {
		t.Fatalf("pide los números: %v", w.last(phone))
	}
	w.write(phone, "Laura Gómez", "1 y 7")
	if !strings.Contains(w.last(phone)["cuerpo"].(string), "No encontré esos números") {
		t.Fatalf("número fuera de la cotización: %v", w.last(phone))
	}
	if o := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+id, nil)).body; o["estado"] != "esperando_aprobacion" {
		t.Fatalf("un número inválido no registra nada: %v", o["estado"])
	}

	w.write(phone, "Laura Gómez", "1 y 3")
	cot := owner.must(http.StatusOK, owner.do("GET", q, nil)).body["cotizacion"].(map[string]any)
	o := owner.must(http.StatusOK, owner.do("GET", "/api/v1/orders/"+id, nil)).body
	if o["estado"] != "en_reparacion" || cot["canalRespuesta"] != "whatsapp" {
		t.Fatalf("aprobada en parte: orden %v, cotización %v", o["estado"], cot)
	}
	approved := map[string]bool{}
	for _, l := range cot["lineas"].([]any) {
		l := l.(map[string]any)
		if a, _ := l["aprobada"].(bool); a {
			approved[l["descripcion"].(string)] = true
		}
	}
	if len(approved) != 2 || !approved["Cambio de pastillas"] || !approved["Lavado de motor"] {
		t.Fatalf("ítems aprobados: %v", cot["lineas"])
	}
	reply := w.last(phone)["cuerpo"].(string)
	total := cot["total"].(float64)
	if !strings.Contains(reply, "aprobaste 2 de 3 ítems") || total >= 200000 || total <= 0 {
		t.Fatalf("total recalculado %v y respuesta %q", total, reply)
	}

	// Con un solo ítem no aparece "Elegir ítems".
	id2, part := w.orderFor(cid, "ITM831")
	w.move(id2, "recibido", nil)
	w.move(id2, "en_diagnostico", nil)
	owner.must(http.StatusOK, owner.do("POST", "/api/v1/orders/"+id2+"/quote/lines", map[string]any{"tipo": "repuesto", "refId": part, "cantidad": 1}))
	owner.must(http.StatusOK, owner.do("POST", "/api/v1/orders/"+id2+"/quote/send", nil))
	w.pump()
	for _, m := range w.sentBy(phone, "AU-02") {
		if hasPrefix(buttons(m), "quote:aprobar:"+id2) && hasPrefix(buttons(m), "quote:elegir:") {
			t.Fatalf("un solo ítem no ofrece elegir: %v", buttons(m))
		}
	}
}
