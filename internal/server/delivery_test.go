package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"sweveh/internal/platform/testdb"
)

// HU-25: entregar pide quién recoge y la confirmación de accesorios; la orden se cierra, sale de las abiertas,
// entra al cálculo de ticket y tiempo, y la bitácora guarda quién recogió y qué se devolvió.
func TestDelivery_PickupAccessoriesAndClose(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550380", "Pablo Rodríguez")
	id, _ := orderInDiagnosis(t, c, "ENT381", 1)
	base := "/api/v1/orders/" + id
	tr := func(body map[string]any) resp {
		body["version"] = r2v(c, id)
		return c.do("POST", base+"/transition", body)
	}
	c.must(http.StatusOK, c.do("PUT", base+"/checklist", map[string]any{"accesorios": []string{"Gato", "Documentos"}}))
	q := base + "/quote"
	c.must(http.StatusOK, c.do("POST", q+"/lines", map[string]any{"tipo": "servicio", "descripcion": "Mano de obra", "cantidad": 1, "precioUnitario": 50000}))
	c.must(http.StatusOK, c.do("POST", q+"/send", nil))
	c.must(http.StatusOK, c.do("POST", q+"/response", map[string]any{"decision": "aprobar", "canal": "mostrador"}))
	c.must(http.StatusOK, tr(map[string]any{"estado": "listo"}))

	if r := tr(map[string]any{"estado": "entregado", "accesoriosConfirmados": true}); r.str("code") != "entrega_incompleta" {
		t.Fatalf("sin quién recoge: %d %v", r.status, r.body)
	}
	if r := tr(map[string]any{"estado": "entregado", "recogidoPor": "Laura Gómez"}); r.str("code") != "entrega_incompleta" {
		t.Fatalf("sin confirmar accesorios: %d %v", r.status, r.body)
	}
	if r := tr(map[string]any{"estado": "entregado", "recogidoPor": "  ", "accesoriosConfirmados": true}); r.str("code") != "entrega_incompleta" {
		t.Fatalf("nombre en blanco: %d %v", r.status, r.body)
	}
	r := c.must(http.StatusOK, tr(map[string]any{"estado": "entregado", "recogidoPor": " Laura Gómez ", "accesoriosConfirmados": true}))
	if r.str("estado") != "entregado" {
		t.Fatalf("entregada: %v", r.body)
	}

	// Criterio 2: sale de las abiertas.
	for _, o := range c.must(http.StatusOK, c.do("GET", "/api/v1/orders?abiertas=1", nil)).body["ordenes"].([]any) {
		if o.(map[string]any)["id"] == id {
			t.Fatal("la orden entregada sigue entre las abiertas")
		}
	}
	// Ya cerrada no vuelve a moverse ni acepta cambios en el checklist.
	is4xx(t, "mover una entregada", tr(map[string]any{"estado": "listo"}))
	is4xx(t, "checklist de una entregada", c.do("PUT", base+"/checklist", map[string]any{"accesorios": []string{"Gato"}}))

	// Entra al cálculo de ticket y tiempo (las consultas de los KPIs leen estado y cerrada_en) y queda en la bitácora.
	var cerradas int
	var despues string
	err := e.db.Tx.Do(testdb.As(tallerOf(t, c)), func(ctx context.Context) error {
		if err := queryRow(ctx, `SELECT count(*) FROM orders.orden WHERE estado = 'entregado' AND cerrada_en IS NOT NULL AND entregado_a = 'Laura Gómez'`, &cerradas); err != nil {
			return err
		}
		return queryRow(ctx, `SELECT despues::text FROM audit.evento WHERE accion = 'orden.estado' AND despues->>'estado' = 'entregado'`, &despues)
	})
	if err != nil {
		t.Fatal(err)
	}
	if cerradas != 1 {
		t.Fatalf("órdenes cerradas para los KPIs: %d", cerradas)
	}
	if !strings.Contains(despues, "Laura Gómez") || !strings.Contains(despues, `"accesorios_devueltos": ["gato", "documentos"]`) {
		t.Fatalf("bitácora de la entrega: %s", despues)
	}
}
