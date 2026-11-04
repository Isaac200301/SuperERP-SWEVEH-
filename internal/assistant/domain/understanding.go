package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Intenciones que puede devolver el intérprete (HU-52, HU-58, HU-59).
var Intents = []string{"cotizar", "cita", "estado", "diagnostico", "humano", "fuera_tema", "saludo", "otro"}

// Understanding es la salida del intérprete (IA o reglas) con nombres en español: es el contrato JSON que se le
// pide al modelo (UnderstandingSchema) y que el código valida antes de usar nada (HU-54, RNF-05).
type Understanding struct {
	Intent   string   `json:"intencion"`
	Vehicle  Vehicle  `json:"vehiculo"`
	Plate    string   `json:"placa"`
	Symptom  string   `json:"sintoma"`
	Services []string `json:"servicios"` // nombres exactos de la lista del taller
	Others   []string `json:"otros"`     // lo pedido que no está en la lista: nunca se cotiza, queda para el dueño
	ForDate  string   `json:"fecha"`     // hoy | manana | ""
}

// Vehicle es lo que el cliente dijo de su vehículo. Todo es opcional.
type Vehicle struct {
	Type  string `json:"tipo"` // carro | moto | ""
	Brand string `json:"marca"`
	Line  string `json:"linea"`
	Year  int    `json:"anio"`
}

// ErrInvalidOutput es una salida que no cumple el esquema: se descarta y la tarjeta queda en "Lo contesto yo".
var ErrInvalidOutput = errors.New("la salida de la IA no cumple el esquema")

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOutput, fmt.Sprintf(format, a...))
}

func oneOf(v string, allowed ...string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// ParseUnderstanding valida la salida cruda contra el esquema y la lista del taller (HU-54):
//   - JSON estricto: campos desconocidos, tipos o valores fuera de lo permitido la descartan;
//   - un servicio que no está en la lista no se inventa: pasa a "otros" (HU-52, criterio 4);
//   - la placa solo vale si está escrita en el mensaje del cliente: la IA no puede crear órdenes con placas que no existen.
func ParseUnderstanding(raw []byte, text string, catalog []string) (Understanding, error) {
	var u Understanding
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		return Understanding{}, invalid("%v", err)
	}
	if dec.More() {
		return Understanding{}, invalid("hay datos después del objeto")
	}
	if !oneOf(u.Intent, Intents...) {
		return Understanding{}, invalid("intención %q", u.Intent)
	}
	if !oneOf(u.Vehicle.Type, "", "carro", "moto") {
		return Understanding{}, invalid("tipo de vehículo %q", u.Vehicle.Type)
	}
	if !oneOf(u.ForDate, "", "hoy", "manana") {
		return Understanding{}, invalid("fecha %q", u.ForDate)
	}
	if u.Vehicle.Year != 0 && (u.Vehicle.Year < 1950 || u.Vehicle.Year > 2100) {
		return Understanding{}, invalid("año %d", u.Vehicle.Year)
	}
	for _, s := range []string{u.Vehicle.Brand, u.Vehicle.Line, u.Symptom} {
		if len([]rune(s)) > 300 {
			return Understanding{}, invalid("texto demasiado largo")
		}
	}
	if len(u.Services)+len(u.Others) > 10 {
		return Understanding{}, invalid("demasiados servicios")
	}
	if u.Plate != "" {
		u.Plate = strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(u.Plate))
		if !oneOf(u.Plate, Plates(text)...) {
			u.Plate = ""
		}
	}
	names := map[string]string{}
	for _, c := range catalog {
		names[Normalize(strings.TrimSpace(c))] = c
	}
	var services []string
	others := []string{}
	for _, o := range u.Others {
		if o = strings.TrimSpace(o); o != "" && len([]rune(o)) <= 120 {
			others = append(others, o)
		}
	}
	for _, s := range u.Services {
		if name, ok := names[Normalize(strings.TrimSpace(s))]; ok {
			if !oneOf(name, services...) {
				services = append(services, name)
			}
		} else if s = strings.TrimSpace(s); s != "" && len([]rune(s)) <= 120 {
			others = append(others, s)
		}
	}
	u.Services, u.Others = services, others
	if u.Services == nil {
		u.Services = []string{}
	}
	return u, nil
}

// Intent pasa la interpretación validada a lo que usa el agente.
func (u Understanding) ToIntent(text string) Intent {
	return Intent{Kind: u.Intent, Plate: u.Plate, Wants: Stems(text), ForDate: u.ForDate, Services: u.Services, Others: u.Others,
		Vehicle: u.Vehicle, Symptom: u.Symptom}
}

// UnderstandingSchema es el JSON Schema que se le pasa al modelo (response_format json_schema).
func UnderstandingSchema(catalog []string) map[string]any {
	str := map[string]any{"type": "string"}
	items := map[string]any{"type": "string"} // un enum vacío no es un esquema válido
	if len(catalog) > 0 {
		items["enum"] = append([]string{}, catalog...)
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"intencion", "vehiculo", "placa", "sintoma", "servicios", "otros", "fecha"},
		"properties": map[string]any{
			"intencion": map[string]any{"type": "string", "enum": Intents},
			"vehiculo": map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"tipo", "marca", "linea", "anio"},
				"properties": map[string]any{
					"tipo":  map[string]any{"type": "string", "enum": []string{"", "carro", "moto"}},
					"marca": str, "linea": str, "anio": map[string]any{"type": "integer"},
				},
			},
			"placa": str, "sintoma": str,
			"servicios": map[string]any{"type": "array", "items": items},
			"otros":     map[string]any{"type": "array", "items": str},
			"fecha":     map[string]any{"type": "string", "enum": []string{"", "hoy", "manana"}},
		},
	}
}

// Plates devuelve todas las placas escritas en el texto (ABC123, ABC12D, ABC12), normalizadas.
func Plates(text string) []string {
	var out []string
	for _, m := range rePlate.FindAllStringSubmatch(text, -1) {
		p := strings.ToUpper(m[1] + m[2] + m[3])
		if !oneOf(p, out...) {
			out = append(out, p)
		}
	}
	return out
}
