// Package domain contiene el catálogo de automatizaciones y sus textos (HU-40 a HU-48). Sin I/O.
package domain

import (
	"regexp"
	"strings"

	"sweveh/internal/shared"
)

// Definition es una tarjeta del catálogo (docs/03 §M5).
type Definition struct {
	Code         string
	Name         string
	Trigger      string
	DefaultOn    bool
	ComingSoon   bool
	SendsMessage bool // AU-08 es interna: no cuesta mensajes
	Editable     bool // el dueño puede editar el texto (HU-42)
	// OutsideWindow: el aviso suele salir fuera de la ventana de 24 h (programado), así que en producción el texto
	// es una plantilla aprobada por Meta: cambiarlo pide una aprobación nueva (HU-42, criterio 3; Q-17).
	OutsideWindow bool
	DefaultText  string
	Placeholders []string
}

// Catalog: 5 encendidas, 3 apagadas y AU-09 "próximamente" (escena 10; Q-25). En los textos, {vehiculo} es el tipo
// ("moto" o "carro", HU-42) y {modelo} la marca y línea ("Yamaha NMAX").
var Catalog = []Definition{
	{Code: "AU-01", Name: "Vehículo recibido", Trigger: "La orden pasa a Recibido", DefaultOn: true, SendsMessage: true, Editable: true,
		DefaultText:  "Hola {cliente}, te escribe {taller}. Recibimos tu {vehiculo} {placa} (orden {orden}). Kilometraje: {km}. {salvedades}Te avisamos cada avance.",
		Placeholders: []string{"cliente", "modelo", "taller", "vehiculo", "placa", "orden", "km", "salvedades"}},
	{Code: "AU-02", Name: "Cotización para aprobar", Trigger: "La orden pasa a Esperando aprobación", DefaultOn: true, SendsMessage: true, Editable: true,
		DefaultText:  "Hola {cliente}, esta es la cotización de tu {vehiculo} {placa}:\n{items}\nTotal: {total}. ¿La apruebas?",
		Placeholders: []string{"cliente", "modelo", "vehiculo", "placa", "items", "total", "taller"}},
	{Code: "AU-03", Name: "Vehículo listo", Trigger: "La orden pasa a Listo", DefaultOn: true, SendsMessage: true, Editable: true,
		DefaultText:  "Hola {cliente}, tu {vehiculo} {placa} está listo. Total: {total}. Puedes recogerlo {horario}.{accesorios}",
		Placeholders: []string{"cliente", "modelo", "vehiculo", "placa", "total", "horario", "accesorios", "taller"}},
	{Code: "AU-04", Name: "Agente de WhatsApp", Trigger: "Llega un mensaje de alguien sin orden abierta", DefaultOn: true, SendsMessage: true},
	{Code: "AU-05", Name: "¿Ya está mi vehículo?", Trigger: "Un cliente con orden abierta pregunta por el estado", DefaultOn: true, SendsMessage: true, Editable: true,
		DefaultText:  "Hola {cliente}. Tu {vehiculo} {placa} está {estado}. {prometida}",
		Placeholders: []string{"cliente", "modelo", "vehiculo", "placa", "estado", "prometida", "taller"}},
	{Code: "AU-06", Name: "Recordatorio de la cita", Trigger: "Orden Por recibir con fecha de hoy (en la mañana)", SendsMessage: true, Editable: true,
		OutsideWindow: true,
		DefaultText:  "Hola {cliente}, te esperamos hoy en {taller} con tu {vehiculo} {placa}. Horario: {horario}.",
		Placeholders: []string{"cliente", "modelo", "taller", "vehiculo", "placa", "horario"}},
	{Code: "AU-07", Name: "Vehículo sin recoger", Trigger: "Orden en Listo hace 3 días", SendsMessage: true, Editable: true,
		OutsideWindow: true,
		DefaultText:  "Hola {cliente}, tu {vehiculo} {placa} está listo desde hace {dias} días. Te esperamos en {taller}, {horario}.",
		Placeholders: []string{"cliente", "modelo", "vehiculo", "placa", "dias", "taller", "horario"}},
	{Code: "AU-08", Name: "Repuesto bajo el mínimo", Trigger: "El inventario baja del mínimo (aviso dentro de la app)"},
	{Code: "AU-09", Name: "Seguimiento de garantía", Trigger: "N días después de Entregado", ComingSoon: true},
}

// ReadyDays es el umbral de AU-07 (configurable más adelante; docs/03 dice 3 días).
const ReadyDays = 3

var (
	ErrUnknown     = shared.NotFound("automatizacion_no_encontrada", "Esa automatización no existe.")
	ErrComingSoon  = shared.Rule("automatizacion_proximamente", "Esta automatización llega pronto; todavía no se puede encender.")
	ErrNotEditable = shared.Rule("texto_no_editable", "El texto de esta automatización no se edita.")
	ErrText        = shared.Validation("texto_invalido", "El texto no puede estar vacío ni pasar de 1.000 letras.").WithField("texto")
)

func Find(code string) (Definition, error) {
	for _, d := range Catalog {
		if d.Code == code {
			return d, nil
		}
	}
	return Definition{}, ErrUnknown
}

var rePH = regexp.MustCompile(`\{([a-z_]+)\}`)

// ValidateText revisa que el texto editado solo use los datos que la automatización conoce (HU-42).
func (d Definition) ValidateText(t string) error {
	t = strings.TrimSpace(t)
	if t == "" || len([]rune(t)) > 1000 {
		return ErrText
	}
	for _, m := range rePH.FindAllStringSubmatch(t, -1) {
		ok := false
		for _, p := range d.Placeholders {
			ok = ok || p == m[1]
		}
		if !ok {
			return shared.Validation("dato_desconocido", "El dato {"+m[1]+"} no existe en este aviso.").WithField("texto")
		}
	}
	return nil
}

// Render llena el texto solo con datos reales de la base (no hay texto redactado por IA aquí: RF-18).
func Render(text string, data map[string]string) string {
	return rePH.ReplaceAllStringFunc(text, func(m string) string { return data[m[1:len(m)-1]] })
}
