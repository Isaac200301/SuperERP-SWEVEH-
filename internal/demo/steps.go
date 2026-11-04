package demo

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"sweveh/internal/platform/db"
)

// setup deja el taller como lo dejaría el dueño en el arranque (HU-03, HU-08) con los datos de docs/03.
func (l *loader) setup() error {
	if _, err := l.call(http.StatusOK, "POST", "/api/v1/workshop/onboarding", map[string]any{"nombre": Name, "atiende": "ambos",
		"especialidades": []string{"frenos_suspension"}, "ciudad": "Medellín", "tamano": "2a5", "contratoAceptado": true}); err != nil {
		return err
	}
	week := map[string]any{}
	for _, d := range []string{"lun", "mar", "mie", "jue", "vie"} {
		week[d] = map[string]any{"abre": "07:30", "cierra": "17:30"}
	}
	week["sab"] = map[string]any{"abre": "07:30", "cierra": "13:00"}
	if _, err := l.call(http.StatusOK, "PUT", "/api/v1/workshop/schedule", map[string]any{"horario": week}); err != nil {
		return err
	}
	for _, s := range services {
		body := map[string]any{"nombre": s.Name, "tipoVehiculo": s.Kind}
		if s.Price != nil {
			body["precio"] = s.Price
		}
		if _, err := l.call(http.StatusCreated, "POST", "/api/v1/services", body); err != nil {
			return err
		}
	}
	for _, p := range parts {
		if _, err := l.call(http.StatusCreated, "POST", "/api/v1/parts", map[string]any{"referencia": p.Ref, "nombre": p.Name,
			"tipoVehiculo": p.Kind, "costo": p.Cost, "precioVenta": p.Price, "existencia": p.Stock, "minimo": p.Min}); err != nil {
			return err
		}
	}
	jhon, err := l.call(http.StatusCreated, "POST", "/api/v1/users", map[string]any{"celular": l.phones.Mechanic, "nombre": "Jhon Cardona", "rol": "mecanico"})
	if err != nil {
		return err
	}
	l.ids["jhon"] = jhon["id"].(string)
	if _, err := l.call(http.StatusCreated, "POST", "/api/v1/users", map[string]any{"celular": l.phones.Reception, "nombre": "Yesica Gómez", "rol": "recepcion"}); err != nil {
		return err
	}
	// El simulador es el canal de la demo; WAHA se conecta desde Mi taller (ADR-0027).
	if _, err := l.call(http.StatusOK, "PUT", "/api/v1/whatsapp/channel", map[string]any{"proveedor": "prueba_local"}); err != nil {
		return err
	}
	for _, c := range l.customers() {
		r, err := l.call(http.StatusCreated, "POST", "/api/v1/customers", map[string]any{"celular": c.Phone, "nombre": c.Name})
		if err != nil {
			return err
		}
		id := r["cliente"].(map[string]any)["id"].(string)
		l.ids[c.Name] = id
		// Autorizaron en el mostrador (HU-86): sin autorización no les salen los avisos.
		if _, err := l.call(http.StatusOK, "POST", "/api/v1/customers/"+id+"/consent", map[string]any{"acepta": true}); err != nil {
			return err
		}
	}
	for _, v := range vehicles {
		body := map[string]any{"placa": v.Plate, "clienteId": l.ids[v.Owner], "marca": v.Brand, "linea": v.Line, "modeloAnio": v.Year}
		for k, x := range v.Engine {
			body[k] = x
		}
		r, err := l.call(http.StatusCreated, "POST", "/api/v1/vehicles", body)
		if err != nil {
			return err
		}
		l.ids[v.Plate] = r["vehiculo"].(map[string]any)["id"].(string)
	}
	return nil
}

// setCounter fija el último número de orden: los números del guion (OS-0031, OS-0038, OS-0040 a OS-0044) quedan
// donde los cita docs/03 aunque se carguen en otro orden. Es la única escritura directa de la carga y respeta RLS.
func (l *loader) setCounter(last int) error {
	return l.boot.DoWithSettings(context.Background(), map[string]string{"app.taller_id": l.tallerID.String()}, func(ctx context.Context) error {
		_, err := db.Exec(ctx, `INSERT INTO orders.contador (taller_id, ultimo) VALUES ($1, $2)
			ON CONFLICT (taller_id) DO UPDATE SET ultimo = EXCLUDED.ultimo`, l.tallerID, last)
		return err
	})
}

func (l *loader) version(id string) (any, error) {
	r, err := l.call(http.StatusOK, "GET", "/api/v1/orders/"+id, nil)
	return r["version"], err
}

func (l *loader) move(id, to string, extra map[string]any) error {
	v, err := l.version(id)
	if err != nil {
		return err
	}
	body := map[string]any{"estado": to, "version": v}
	for k, x := range extra {
		body[k] = x
	}
	_, err = l.call(http.StatusOK, "POST", "/api/v1/orders/"+id+"/transition", body)
	return err
}

// openTo abre la orden recibida y la lleva al estado pedido (recibido o en_diagnostico).
func (l *loader) openTo(plate, symptom, state string) (string, error) {
	return l.open(plate, symptom, state, false)
}

func (l *loader) open(plate, symptom, state string, example bool) (string, error) {
	r, err := l.call(http.StatusCreated, "POST", "/api/v1/orders", map[string]any{"placa": plate, "sintoma": symptom, "estadoInicial": "recibido",
		"ejemplo": example})
	if err != nil {
		return "", err
	}
	id := r["id"].(string)
	if state == "en_diagnostico" {
		err = l.move(id, "en_diagnostico", nil)
	}
	return id, err
}

func (l *loader) quote(id, desc string, price int) error {
	_, err := l.call(http.StatusOK, "POST", "/api/v1/orders/"+id+"/quote/lines", map[string]any{"tipo": "servicio", "descripcion": desc,
		"cantidad": 1, "precioUnitario": price})
	return err
}

// approved cotiza, envía y registra la aprobación en el mostrador: la orden queda en reparación.
func (l *loader) approved(id, desc string, price int) error {
	if err := l.quote(id, desc, price); err != nil {
		return err
	}
	if _, err := l.call(http.StatusOK, "POST", "/api/v1/orders/"+id+"/quote/send", nil); err != nil {
		return err
	}
	_, err := l.call(http.StatusOK, "POST", "/api/v1/orders/"+id+"/quote/response", map[string]any{"decision": "aprobar", "canal": "mostrador"})
	return err
}

func (l *loader) openToRepair(plate, desc string, price int, at time.Time) (string, error) {
	return l.repair(plate, desc, price, at, false)
}

func (l *loader) repair(plate, desc string, price int, at time.Time, example bool) (string, error) {
	if err := l.at(at); err != nil {
		return "", err
	}
	id, err := l.open(plate, desc, "en_diagnostico", example)
	if err != nil {
		return "", err
	}
	return id, l.approved(id, desc, price)
}

// deliveredOrder es una orden completa: se abre en at y se entrega dur después.
func (l *loader) deliveredOrder(orders map[string]string, code, plate, desc string, price int, at time.Time, dur time.Duration, example bool) error {
	id, err := l.repair(plate, desc, price, at, example)
	if err != nil {
		return fmt.Errorf("%s: %w", code, err)
	}
	if err := l.at(at.Add(dur)); err != nil {
		return err
	}
	if err := l.move(id, "listo", nil); err != nil {
		return fmt.Errorf("%s: %w", code, err)
	}
	if err := l.move(id, "entregado", map[string]any{"recogidoPor": "El cliente", "accesoriosConfirmados": true}); err != nil {
		return fmt.Errorf("%s: %w", code, err)
	}
	orders[code] = id
	return nil
}

// assign pone a Jhon y la fecha prometida (en una orden "Por recibir" es la cita: AU-06 la recuerda).
func (l *loader) assign(id string, when time.Time) error {
	_, err := l.call(http.StatusOK, "PATCH", "/api/v1/orders/"+id+"/assignment", map[string]any{"mecanicoId": l.ids["jhon"],
		"fechaPrometida": when.UTC().Format(time.RFC3339)})
	return err
}
