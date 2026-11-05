package server_test

import (
	"net/http"
	"testing"
)

// HU-92: el dueño sube clientes y vehículos con la plantilla. La vista previa marca cada fila con error y no guarda
// nada; con todo bien se importan en bloque: los clientes nuevos quedan con autorización pendiente (HU-95), el que
// ya existía por celular se reutiliza sin cambiarle el nombre, y la fila sin cilindraje ni turbo entra con motor
// pendiente. Una placa que ya está en el taller es error. El mecánico no importa.
func TestImport_CustomersAndVehicles(t *testing.T) {
	e := newEnv(t)
	c := e.register("3105550710", "Rosa Pérez")
	c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550711", "nombre": "Laura Gómez"}))
	r := c.must(http.StatusCreated, c.do("POST", "/api/v1/customers", map[string]any{"celular": "3005550719", "nombre": "Dueña Anterior"}))
	c.must(http.StatusCreated, c.do("POST", "/api/v1/vehicles", map[string]any{"placa": "OLD719", "clienteId": nested(r.body, "cliente", "id")}))

	malo := "Nombre;Celular;Placa;Tipo de vehículo;Marca;Cilindraje;Turbo\n" +
		"Carlos Ruiz;3005550712;IMP712;carro;Mazda;2000;no\n" +
		"Ana Ríos;3005550713;XYZ12A;carro;;;\n" + // placa de moto marcada como carro
		"Pedro Gil;3005550714;OLD719;carro;;;\n" // ya está en el taller
	r = c.must(http.StatusOK, c.uploadCSV("/api/v1/vehicles/import", malo))
	if r.body["conError"] != float64(2) || r.body["importadas"] != float64(0) {
		t.Fatalf("vista previa: %v", r.body)
	}
	if r = c.uploadCSV("/api/v1/vehicles/import?confirmar=1", malo); r.status != http.StatusUnprocessableEntity {
		t.Fatalf("confirmar con errores: %d %v", r.status, r.body)
	}
	if r := c.do("GET", "/api/v1/vehicles/by-plate/IMP712", nil); r.status != http.StatusNotFound {
		t.Fatalf("con errores no debía importarse nada: %d", r.status)
	}

	bueno := "Nombre,Celular,Placa,Tipo,Marca,Línea,Modelo,Cilindraje,Turbo,Energía,Caja\n" +
		"Carlos Ruiz,300 555 0712,IMP712,Carro,Mazda,3,2015,2000,no,Gasolina,Automática\n" +
		"Carlos Ruiz,3005550712,IMP71F,moto,Yamaha,NMAX,2022,,,,\n" + // mismo cliente, moto sin motor
		"Laura G.,3005550711,LAU711,carro,,,,,,,\n" // ya existe por celular: se reutiliza
	r = c.must(http.StatusOK, c.uploadCSV("/api/v1/vehicles/import?confirmar=1", bueno))
	if r.body["importadas"] != float64(3) || r.body["clientesNuevos"] != float64(1) {
		t.Fatalf("importar: %v", r.body)
	}
	carlos := c.must(http.StatusOK, c.do("GET", "/api/v1/customers/by-phone/3005550712", nil)).body
	if carlos["nombre"] != "Carlos Ruiz" || carlos["autorizacionEstado"] != "pendiente" {
		t.Fatalf("cliente importado: %v", carlos)
	}
	vs := c.must(http.StatusOK, c.do("GET", "/api/v1/customers/"+carlos["id"].(string)+"/vehicles", nil)).body["vehiculos"].([]any)
	if len(vs) != 2 {
		t.Fatalf("vehículos de Carlos: %v", vs)
	}
	for _, v := range vs {
		v := v.(map[string]any)
		if pending := v["placa"] == "IMP71F"; v["motorPendiente"] != pending {
			t.Errorf("%s: motor pendiente %v", v["placa"], v["motorPendiente"])
		}
	}
	if laura := c.must(http.StatusOK, c.do("GET", "/api/v1/customers/by-phone/3005550711", nil)).body; laura["nombre"] != "Laura Gómez" {
		t.Fatalf("el nombre de un cliente que ya existía no cambia: %v", laura["nombre"])
	}

	c.must(http.StatusCreated, c.do("POST", "/api/v1/users", map[string]any{"celular": "3005550715", "nombre": "Mecánico", "rol": "mecanico"}))
	if r := e.login("3005550715").uploadCSV("/api/v1/vehicles/import", bueno); r.status != http.StatusForbidden {
		t.Fatalf("el mecánico importa: %d", r.status)
	}
}
