package demo

import (
	"fmt"
	"net/http"
	"time"
)

// Datos de docs/03 "El taller ficticio de la demo". Con HU-98 el taller atiende carros y motos.

type service struct {
	Name  string
	Price any // nil = sin precio cargado (el borrador dice "el taller te confirma el valor")
	Kind  string
}

var services = []service{
	{"Revisión de frenos", 40000, "ambos"},
	{"Cambio de pastillas delanteras", 60000, "ambos"},
	{"Cambio de aceite y filtro", 25000, "ambos"},
	{"Diagnóstico con escáner", 70000, "ambos"},
	{"Alineación y balanceo", 80000, "carro"},
	{"Revisión antes de técnico-mecánica", 90000, "ambos"},
	{"Cambio de amortiguadores (par)", nil, "carro"},
}

type part struct {
	Ref, Name, Kind         string
	Cost, Price, Stock, Min int
}

// El filtro queda justo en el mínimo y el bombillo agotado: alimentan AU-08 y "Hoy tienes pendiente".
var parts = []part{
	{"PF-SPK", "Pastillas delanteras Chevrolet Spark", "carro", 55000, 85000, 4, 2},
	{"AC-2050", "Aceite 20W-50 (cuarto)", "ambos", 22000, 32000, 12, 6},
	{"FA-SPK", "Filtro de aceite Spark / Aveo", "carro", 14000, 22000, 3, 3},
	{"LF-DOT4", "Líquido de frenos DOT 4", "ambos", 18000, 28000, 6, 2},
	{"BH-H4", "Bombillo H4", "carro", 9000, 15000, 0, 2},
}

type vehicle struct {
	Owner, Plate, Brand, Line string
	Year                      int
	Engine                    map[string]any // nil = motor pendiente (HU-97, criterio 2)
}

func carEngine(cc string) map[string]any {
	return map[string]any{"cilindraje": cc, "energia": "gasolina", "aspiracion": "atmosferico", "transmision": "manual"}
}

// Los 5 clientes (Laura no existe todavía: escribe en vivo en la escena 4).
func (l *loader) customers() []struct{ Name, Phone string } {
	p := l.phones
	return []struct{ Name, Phone string }{
		{"Andrés Restrepo", p.B}, {"Diego Zapata", p.Diego}, {"Marta Ospina", p.Marta}, {"Carlos Mejía", p.Carlos}, {"Sebastián Arango", p.D},
	}
}

var vehicles = []vehicle{
	{"Andrés Restrepo", "HTR315", "Renault", "Logan", 2018, carEngine("1600")},
	{"Diego Zapata", "GBW118", "Nissan", "March", 2019, carEngine("1600")},
	{"Marta Ospina", "JNX907", "Mazda", "2", 2020, carEngine("1500")},
	{"Carlos Mejía", "FQD264", "Kia", "Picanto", 2017, carEngine("1000")},
	{"Sebastián Arango", "XVN43F", "Honda", "CB 190R", 2021, map[string]any{"cilindraje": "190", "energia": "gasolina", "ciclo": "4t",
		"refrigeracion": "aire", "transmision": "manual"}},
	// HU-97, criterio 2: un vehículo queda con "motor pendiente" para mostrar el caso. Va en uno sin órdenes, porque
	// el sistema no deja recibir un vehículo con el motor pendiente.
	{"Marta Ospina", "WMR552", "Chevrolet", "Aveo", 2012, nil},
}

// load carga el taller en orden cronológico: el reloj de la carga solo avanza.
func (l *loader) load(today time.Time, orders map[string]string) error {
	if err := l.setup(); err != nil {
		return err
	}
	settings, err := l.call(http.StatusOK, "GET", "/api/v1/workshop/settings", nil)
	if err != nil {
		return err
	}
	// Durante la carga la recepción no pide fotos ni kilometraje; al final se dejan como estaban (escena 5: Jhon toma
	// las fotos de la orden de Laura).
	relaxed := copyMap(settings)
	relaxed["fotosMinimas"], relaxed["kmObligatorio"] = 0, false
	if _, err := l.call(http.StatusOK, "PUT", "/api/v1/workshop/settings", relaxed); err != nil {
		return err
	}

	// Carlos vuelve: dos visitas entregadas (historial y "clientes que vuelven").
	if err := l.setCounter(30); err != nil {
		return err
	}
	if err := l.deliveredOrder(orders, "OS-0031", "FQD264", "Cambio de aceite y filtro", 25000, today.AddDate(0, 0, -240).Add(9*time.Hour), 6*time.Hour, false); err != nil {
		return err
	}
	if err := l.setCounter(37); err != nil {
		return err
	}
	if err := l.deliveredOrder(orders, "OS-0038", "FQD264", "Revisión de frenos", 40000, today.AddDate(0, 0, -60).Add(9*time.Hour), 26*time.Hour, false); err != nil {
		return err
	}

	// Unas 20 órdenes entregadas de los últimos 30 días, marcadas como ejemplo, para que los KPIs no salgan en cero.
	if err := l.setCounter(10); err != nil {
		return err
	}
	plates := []string{"HTR315", "GBW118", "JNX907", "XVN43F"}
	offsets := []int{30, 29, 28, 26, 25, 24, 22, 21, 20, 18, 17, 16, 14, 13, 12, 10, 9, 8, 7, 6}
	for i, off := range offsets {
		s := services[i%6] // los 6 servicios con precio
		hours := time.Duration(4+(i*7)%40) * time.Hour
		code := fmt.Sprintf("ejemplo-%02d", i+1)
		if err := l.deliveredOrder(orders, code, plates[i%len(plates)], s.Name, s.Price.(int), today.AddDate(0, 0, -off).Add(8*time.Hour), hours, true); err != nil {
			return err
		}
	}

	// OS-0040 a OS-0044, el estado al empezar la demo.
	if err := l.setCounter(39); err != nil {
		return err
	}
	// Marta: listo hace 4 días, sin recoger (AU-07, escena 8).
	id, err := l.openToRepair("JNX907", "Revisión antes de técnico-mecánica", 90000, today.AddDate(0, 0, -5).Add(10*time.Hour))
	if err != nil {
		return err
	}
	if err := l.at(l.now.Add(-4*24*time.Hour - 2*time.Hour)); err != nil { // "4 días sin recoger" a cualquier hora del día
		return err
	}
	if err := l.move(id, "listo", nil); err != nil {
		return err
	}
	orders["OS-0040"] = id

	// Sebastián: la moto llega hoy (escena 5-M): orden "Por recibir" con la cita de hoy y Jhon asignado.
	if err := l.at(l.now.Add(-48 * time.Hour)); err != nil {
		return err
	}
	moto, err := l.call(http.StatusCreated, "POST", "/api/v1/orders", map[string]any{"placa": "XVN43F", "sintoma": "Mantenimiento: cambio de aceite y revisión de frenos"})
	if err != nil {
		return err
	}
	orders["OS-0041"] = moto["id"].(string)
	if err := l.assign(orders["OS-0041"], today.Add(10*time.Hour)); err != nil {
		return err
	}

	// Carlos: tercera visita, en diagnóstico.
	if err := l.at(l.now.Add(-30 * time.Hour)); err != nil {
		return err
	}
	if orders["OS-0042"], err = l.openTo("FQD264", "Testigo de motor encendido", "en_diagnostico"); err != nil {
		return err
	}

	// Andrés: en reparación, prometido hoy a las 5:00 p. m., con la última foto del trabajo (escena 7, AU-05).
	if err := l.at(l.now.Add(-29 * time.Hour)); err != nil {
		return err
	}
	if orders["OS-0043"], err = l.openToRepair("HTR315", "Cambio de pastillas delanteras", 60000, l.clk.Now()); err != nil {
		return err
	}
	if err := l.assign(orders["OS-0043"], today.Add(17*time.Hour)); err != nil {
		return err
	}
	if err := l.upload(orders["OS-0043"], "frente"); err != nil {
		return err
	}

	// Diego: esperando aprobación hace 26 horas (KPI de aprobaciones, HU-69).
	if err := l.at(l.now.Add(-28 * time.Hour)); err != nil {
		return err
	}
	if orders["OS-0044"], err = l.openTo("GBW118", "Ruido en la suspensión delantera", "en_diagnostico"); err != nil {
		return err
	}
	if err := l.quote(orders["OS-0044"], "Revisión de frenos", 40000); err != nil {
		return err
	}
	if err := l.at(l.now.Add(-26 * time.Hour)); err != nil {
		return err
	}
	if _, err := l.call(http.StatusOK, "POST", "/api/v1/orders/"+orders["OS-0044"]+"/quote/send", nil); err != nil {
		return err
	}

	// Al final, la recepción vuelve a pedir lo que pedía.
	if err := l.at(l.now); err != nil {
		return err
	}
	current, err := l.call(http.StatusOK, "GET", "/api/v1/workshop/settings", nil)
	if err != nil {
		return err
	}
	settings["version"] = current["version"]
	_, err = l.call(http.StatusOK, "PUT", "/api/v1/workshop/settings", settings)
	return err
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
