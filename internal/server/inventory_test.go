package server_test

// Bloque 2: inventario completo de punta a punta (HU-32, HU-34, HU-35, HU-37). Datos Ficticio.

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
)

// uploadCSV envía el archivo de importación como lo hace la pantalla de repuestos.
func (c *client) uploadCSV(path, csv string) resp {
	c.e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("archivo", "repuestos.csv")
	_, _ = io.WriteString(fw, csv)
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

func TestInventory_SheetSearchMovementsAndImport(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550190", "Óscar Ruiz")

	// HU-32 con la ficha: marca, categoría, ubicación y referencia original.
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/parts", map[string]any{
		"referencia": "PF-20", "nombre": "Pastillas delanteras", "tipoVehiculo": "carro", "costo": 40000, "precioVenta": 65000,
		"existencia": 2, "minimo": 2, "marca": "Bosch", "categoria": "frenos", "ubicacion": "Estante A3", "referenciaOem": "0986 494",
	}))
	id, ver := r.str("id"), r.body["version"]
	if r.str("referenciaOem") != "0986-494" || r.str("categoria") != "frenos" || r.body["bajoMinimo"] != true {
		t.Fatalf("repuesto: %v", r.body)
	}
	is4xx(t, "categoría inválida", c.do("POST", "/api/v1/parts", map[string]any{"referencia": "X1", "nombre": "x", "precioVenta": 1, "categoria": "radios"}))

	// HU-34: se encuentra por la referencia original y filtra por categoría.
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/parts?q=0986", nil))
	if l := r.body["repuestos"].([]any); len(l) != 1 {
		t.Fatalf("buscar por OEM: %v", r.body)
	}
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/parts?categoria=filtros", nil))
	if l := r.body["repuestos"].([]any); len(l) != 0 {
		t.Fatalf("filtro por categoría: %v", r.body)
	}

	// Editar con versión: mover de estante y bajar el mínimo.
	r = c.must(http.StatusOK, c.do("PATCH", "/api/v1/parts/"+id, map[string]any{"ubicacion": "Bodega 2", "minimo": 1, "version": ver}))
	if r.str("ubicacion") != "Bodega 2" || r.str("marca") != "Bosch" || r.body["bajoMinimo"] != false {
		t.Fatalf("editar: %v", r.body)
	}

	// HU-35: la entrada queda en los movimientos, después de la existencia inicial.
	c.must(http.StatusOK, c.do("POST", "/api/v1/parts/"+id+"/entries", map[string]any{"cantidad": 5}))
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/parts/"+id+"/movements", nil))
	movs := r.body["movimientos"].([]any)
	if len(movs) != 2 || movs[0].(map[string]any)["cantidad"] != float64(5) || movs[1].(map[string]any)["motivo"] != "entrada" {
		t.Fatalf("movimientos: %v", movs)
	}

	// HU-37: la vista previa marca errores y no importa; confirmar con errores es 422 y tampoco importa.
	malo := "referencia;nombre;precio;existencia;categoria\nFA-10;Filtro de aceite;20000;10;filtros\nPF-20;Ya existe;1000;;\nBH-1;Bombillo;;;\n"
	r = c.must(http.StatusOK, c.uploadCSV("/api/v1/parts/import", malo))
	if r.body["conError"] != float64(2) || r.body["importadas"] != float64(0) {
		t.Fatalf("vista previa: %v", r.body)
	}
	if r = c.uploadCSV("/api/v1/parts/import?confirmar=1", malo); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("confirmar con errores: %d %v", r.status, r.body)
	}
	if l := c.must(http.StatusOK, c.do("GET", "/api/v1/parts?q=FA-10", nil)).body["repuestos"].([]any); len(l) != 0 {
		t.Fatal("con errores no debía importarse nada")
	}
	bueno := "referencia,nombre,precio,existencia,categoria\nFA-10,Filtro de aceite,20000,10,filtros\nBH-1,Bombillo H4,8000,,electrico\n"
	r = c.must(http.StatusOK, c.uploadCSV("/api/v1/parts/import?confirmar=1", bueno))
	if r.body["importadas"] != float64(2) {
		t.Fatalf("importar: %v", r.body)
	}
	fa := c.must(http.StatusOK, c.do("GET", "/api/v1/parts?q=FA-10", nil)).body["repuestos"].([]any)[0].(map[string]any)
	r = c.must(http.StatusOK, c.do("GET", "/api/v1/parts/"+fa["id"].(string)+"/movements", nil))
	if m := r.body["movimientos"].([]any); fa["existencia"] != float64(10) || len(m) != 1 || m[0].(map[string]any)["motivo"] != "importacion" {
		t.Fatalf("importado con su movimiento: %v %v", fa, r.body)
	}

	// Permisos y aislamiento: el mecánico no importa; otro taller no ve los movimientos.
	c.must(http.StatusCreated, c.do("POST", "/api/v1/users", map[string]any{"celular": "3005550191", "nombre": "Mecánico", "rol": "mecanico"}))
	if r := e.login("3005550191").uploadCSV("/api/v1/parts/import", bueno); r.status != http.StatusForbidden {
		t.Fatalf("el mecánico importa: %d", r.status)
	}
	other := e.register("3105550199", "Otro Taller")
	if r := other.do("GET", "/api/v1/parts/"+id+"/movements", nil); r.status != http.StatusNotFound {
		t.Fatalf("otro taller ve movimientos: %d", r.status)
	}
}

// El archivo de prueba de Pablo (3-nov-2026), tal como lo exportó: encabezados de la pantalla, "Para" y
// categorías en texto. Debe pasar completo: antes se rechazaba entero por no reconocer "Precio de venta (COP)".
func TestInventory_ImportsPablosSpreadsheet(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550195", "Pablo Rodríguez")
	data, err := os.ReadFile("testdata/inventario-repuestos-prueba.csv")
	if err != nil {
		t.Fatal(err)
	}
	r := c.must(http.StatusOK, c.uploadCSV("/api/v1/parts/import", string(data)))
	filas := r.body["filas"].([]any)
	for _, f := range filas {
		if m := f.(map[string]any); m["estado"] != "ok" {
			t.Errorf("fila %v con error: %v", m["fila"], m["errores"])
		}
	}
	if r.body["conError"] != float64(0) || len(filas) != 95 {
		t.Fatalf("vista previa: %d filas, %v con error", len(filas), r.body["conError"])
	}
	r = c.must(http.StatusOK, c.uploadCSV("/api/v1/parts/import?confirmar=1", string(data)))
	if r.body["importadas"] != float64(95) {
		t.Fatalf("importar: %v", r.body["importadas"])
	}
	p := c.must(http.StatusOK, c.do("GET", "/api/v1/parts?q=CAJ-0001", nil)).body["repuestos"].([]any)[0].(map[string]any)
	if p["categoria"] != "transmision" || p["tipoVehiculo"] != "carro" || p["precioVenta"] != float64(580000) || p["existencia"] != float64(2) {
		t.Fatalf("repuesto importado: %v", p)
	}
	low := c.must(http.StatusOK, c.do("GET", "/api/v1/parts/low", nil)).body["repuestos"].([]any)
	if len(low) == 0 {
		t.Fatal("LUB-0005 (3 de mínimo 10) y FRE-0008 (agotado) debían quedar bajo el mínimo")
	}
}
