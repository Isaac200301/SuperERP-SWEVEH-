package app

import (
	"context"
	"strings"

	"sweveh/internal/automations/domain"
	"sweveh/internal/shared"
)

var system = shared.Actor{Type: shared.ActorAgent}

// fire ejecuta una automatización de mensaje para una orden, una sola vez por clave (HU-46, HU-48).
// Reglas: encendida, cliente con autorización (HU-95, RF-22), orden que no sea de ejemplo (HU-05),
// dentro de la ventana de 24 h o con plantilla (HU-47).
func (s *Service) fire(ctx context.Context, tallerID shared.ID, code, key string, o OrderInfo, build func(Workshop, Customer, string) (string, []Button, error)) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		d, err := domain.Find(code)
		if err != nil {
			return err
		}
		rules, err := s.repo.Rules(ctx, tallerID)
		if err != nil {
			return err
		}
		if !s.active(rules, d) {
			return nil
		}
		done, err := s.repo.HasRun(ctx, tallerID, code, key)
		if err != nil || done {
			return err
		}
		oid, cid := o.ID, o.CustomerID
		skip := func(why string) error {
			_, err := s.repo.RecordRun(ctx, tallerID, code, key, &oid, &cid, "omitido", why, nil)
			return err
		}
		if o.Example {
			return skip("orden de ejemplo: no se envía nada")
		}
		c, err := s.customers.Get(ctx, tallerID, o.CustomerID)
		if err != nil {
			return err
		}
		if c.Consent != "autorizada" {
			return skip("el cliente no ha autorizado el tratamiento de datos")
		}
		w, err := s.workshops.Get(ctx, tallerID)
		if err != nil {
			return err
		}
		text, buttons, err := build(w, c, s.text(rules, d))
		if err != nil {
			return err
		}
		msgID, closed, err := s.msg.Send(ctx, Outgoing{TallerID: tallerID, Phone: c.Phone, Text: text, Buttons: buttons, OrderID: &oid, Automation: code})
		if closed {
			return skip("fuera de la ventana de 24 h: necesita plantilla aprobada por Meta (Q-07)")
		}
		if err != nil {
			return err
		}
		_, err = s.repo.RecordRun(ctx, tallerID, code, key, &oid, &cid, "enviado", "", msgID)
		return err
	})
}

func (s *Service) base(w Workshop, c Customer, o OrderInfo) map[string]string {
	return map[string]string{"cliente": firstName(c.Name), "taller": w.Name, "vehiculo": o.Vehicle, "placa": o.Plate, "orden": o.Code}
}

func firstName(n string) string {
	if f := strings.Fields(n); len(f) > 0 {
		return f[0]
	}
	return n
}

// OnStateChanged reacciona al cambio de estado de la orden: AU-01 (Recibido) y AU-03 (Listo) (HU-48).
func (s *Service) OnStateChanged(ctx context.Context, tallerID, orderID shared.ID, to, eventKey string) error {
	var o OrderInfo
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		o, err = s.orders.Info(ctx, tallerID, orderID)
		return err
	}); err != nil {
		return err
	}
	switch to {
	case "recibido":
		return s.fire(ctx, tallerID, "AU-01", eventKey, o, func(w Workshop, c Customer, t string) (string, []Button, error) {
			d := s.base(w, c, o)
			d["km"] = "sin registrar"
			if o.Km != nil {
				d["km"] = itoa(*o.Km)
			}
			if len(o.Exceptions) > 0 {
				d["salvedades"] = "Anotamos: " + strings.Join(o.Exceptions, "; ") + ". "
			}
			return domain.Render(t, d), nil, nil
		})
	case "listo":
		return s.fire(ctx, tallerID, "AU-03", eventKey, o, func(w Workshop, c Customer, t string) (string, []Button, error) {
			d := s.base(w, c, o)
			d["horario"] = s.ScheduleText(w, s.clock.Now())
			d["total"] = "por confirmar"
			if q, err := s.quotes.ForOrder(ctx, tallerID, orderID); err != nil {
				return "", nil, err
			} else if q != nil {
				d["total"] = Money(q.Total)
			}
			if len(o.Accessories) > 0 {
				d["accesorios"] = " Te devolvemos: " + strings.Join(o.Accessories, ", ") + "."
			}
			return domain.Render(t, d), nil, nil
		})
	}
	return nil
}

// OnQuoteSent envía la cotización con botones Aprobar / Tengo dudas (AU-02, HU-28).
func (s *Service) OnQuoteSent(ctx context.Context, tallerID, orderID shared.ID, eventKey string) error {
	var o OrderInfo
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		o, err = s.orders.Info(ctx, tallerID, orderID)
		return err
	}); err != nil {
		return err
	}
	return s.fire(ctx, tallerID, "AU-02", eventKey, o, func(w Workshop, c Customer, t string) (string, []Button, error) {
		q, err := s.quotes.ForOrder(ctx, tallerID, orderID)
		if err != nil {
			return "", nil, err
		}
		d := s.base(w, c, o)
		if q != nil {
			d["items"] = "• " + strings.Join(q.Lines, "\n• ")
			d["total"] = Money(q.Total)
		}
		id := orderID.String()
		return domain.Render(t, d), []Button{{ID: "quote:aprobar:" + id, Title: "Aprobar"}, {ID: "quote:dudas:" + id, Title: "Tengo dudas"}}, nil
	})
}

// StatusReply arma la respuesta de AU-05 con el estado real (HU-56). La usa el agente; no es texto de IA.
func (s *Service) StatusReply(ctx context.Context, tallerID shared.ID, o OrderInfo, c Customer) (string, bool, error) {
	var text string
	var on bool
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		rules, err := s.repo.Rules(ctx, tallerID)
		if err != nil {
			return err
		}
		d, _ := domain.Find("AU-05")
		on = s.active(rules, d)
		w, err := s.workshops.Get(ctx, tallerID)
		if err != nil {
			return err
		}
		data := s.base(w, c, o)
		data["estado"] = stateWords[o.State]
		if o.PromisedAt != nil && o.State != "listo" && o.State != "entregado" {
			data["prometida"] = "Te lo tenemos prometido para " + s.FormatDate(*o.PromisedAt, s.clock.Now()) + "."
		} else if o.State == "listo" {
			data["prometida"] = "Puedes recogerlo " + s.ScheduleText(w, s.clock.Now()) + "."
		}
		text = strings.TrimSpace(domain.Render(s.text(rules, d), data))
		return nil
	})
	return text, on, err
}

// LowStock registra el aviso interno AU-08 (no cuesta mensajes; se ve en "Hoy tienes pendiente").
func (s *Service) LowStock(ctx context.Context, tallerID, partID shared.ID, ref string, eventKey string) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		rules, err := s.repo.Rules(ctx, tallerID)
		if err != nil {
			return err
		}
		d, _ := domain.Find("AU-08")
		if !s.active(rules, d) {
			return nil
		}
		_, err = s.repo.RecordRun(ctx, tallerID, "AU-08", eventKey, nil, nil, "enviado", "repuesto "+ref+" en el mínimo o por debajo", nil)
		return err
	})
}

// Daily corre AU-06 (cita de hoy) y AU-07 (listo hace 3 días) en todos los talleres. El worker la llama
// con un candado consultivo para que corra en una sola instancia (R-07).
func (s *Service) Daily(ctx context.Context, ids []shared.ID, withTenant func(context.Context, shared.ID) context.Context) error {
	now := s.clock.Now()
	today := now.In(s.loc).Format("2006-01-02")
	for _, tid := range ids {
		tctx := withTenant(ctx, tid)
		var pending, ready []OrderInfo
		if err := s.uow.Do(tctx, func(ctx context.Context) error {
			var err error
			if pending, err = s.orders.InState(ctx, tid, "por_recibir"); err != nil {
				return err
			}
			ready, err = s.orders.InState(ctx, tid, "listo")
			return err
		}); err != nil {
			return err
		}
		for _, o := range pending {
			if o.PromisedAt == nil || o.PromisedAt.In(s.loc).Format("2006-01-02") != today {
				continue
			}
			o := o
			if err := s.fire(tctx, tid, "AU-06", "cita:"+o.ID.String()+":"+today, o, func(w Workshop, c Customer, t string) (string, []Button, error) {
				d := s.base(w, c, o)
				d["horario"] = s.ScheduleText(w, now)
				return domain.Render(t, d), nil, nil
			}); err != nil {
				return err
			}
		}
		for _, o := range ready {
			daysReady := int(now.Sub(o.StateSince).Hours() / 24)
			if daysReady < domain.ReadyDays {
				continue
			}
			o := o
			if err := s.fire(tctx, tid, "AU-07", "sinrecoger:"+o.ID.String(), o, func(w Workshop, c Customer, t string) (string, []Button, error) {
				d := s.base(w, c, o)
				d["dias"], d["horario"] = itoa(daysReady), s.ScheduleText(w, now)
				return domain.Render(t, d), nil, nil
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
