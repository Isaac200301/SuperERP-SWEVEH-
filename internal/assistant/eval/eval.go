// Package eval mide el intérprete del agente con el juego de 30 mensajes de estilo (HU-102, D-08): intención,
// tipo de vehículo, servicios bien sacados de la lista y precios fuera de lista en el borrador (que deben ser 0).
// Lo usan la prueba con las reglas (CI) y cmd/agent-eval con el proveedor configurado (OpenRouter).
package eval

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
)

//go:embed mensajes.json
var defaultSet []byte

// Item es un servicio o repuesto del taller de prueba.
type Item struct {
	Name    string `json:"nombre"`
	Price   *int64 `json:"precio"`
	Vehicle string `json:"tipo"`
}

// Case es un mensaje con lo que se espera de él.
type Case struct {
	Text       string   `json:"texto"`
	Intent     string   `json:"intencion"`
	Intents    []string `json:"intenciones"` // otras intenciones que también valen
	Vehicle    string   `json:"tipo"`        // "" = no se exige
	Services   []string `json:"servicios"`   // deben salir todos
	Acceptable []string `json:"aceptables"`  // pueden salir además
}

// Set es el juego completo.
type Set struct {
	Workshop struct {
		Name     string `json:"nombre"`
		Attends  string `json:"atiende"`
		Services []Item `json:"servicios"`
		Parts    []Item `json:"repuestos"`
	} `json:"taller"`
	Cases []Case `json:"mensajes"`
}

// Default es el juego de 30 mensajes del repositorio.
func Default() (Set, error) {
	var s Set
	err := json.Unmarshal(defaultSet, &s)
	return s, err
}

func (s Set) priced() (services, parts []domain.Priced) {
	for _, i := range s.Workshop.Services {
		services = append(services, domain.Priced{Name: i.Name, Price: i.Price, Kind: "servicio", Vehicle: i.Vehicle})
	}
	for _, i := range s.Workshop.Parts {
		parts = append(parts, domain.Priced{Name: i.Name, Price: i.Price, Kind: "repuesto", Vehicle: i.Vehicle})
	}
	return services, parts
}

// Result es lo que pasó con un mensaje.
type Result struct {
	Case
	Got                          domain.Understanding
	Err                          error
	IntentOK, VehicleOK, Correct bool // Correct = servicios bien sacados
	Draft                        string
	OutOfList                    bool
	LatencyMs                    int
}

// Report resume el juego.
type Report struct {
	Results                                         []Result
	Provider, Model                                 string
	Intents, Vehicles, Services, Invalid, OutOfList int
	TokensIn, TokensOut                             int
	CostMicroUSD                                    int64
}

// MinServices es el criterio de HU-102: al menos 25 de 30 con el servicio bien sacado.
const MinServices = 25

// Passes dice si el intérprete cumple HU-102: ningún precio fuera de lista y al menos 25 servicios bien.
func (r Report) Passes() bool { return r.OutOfList == 0 && r.Services >= MinServices }

// Run pasa cada mensaje por el intérprete y arma el borrador como lo haría el agente.
func Run(ctx context.Context, it app.Interpreter, s Set) Report {
	services, parts := s.priced()
	items := domain.Items(services, parts)
	catalog := domain.Names(items)
	all := append(append([]domain.Priced{}, services...), parts...)
	var rep Report
	for _, c := range s.Cases {
		r := Result{Case: c}
		start := time.Now()
		cctx, cancel := context.WithTimeout(ctx, app.AITimeout)
		out, err := it.Interpret(cctx, app.InterpretRequest{Text: c.Text, Workshop: s.Workshop.Name, Attends: s.Workshop.Attends, Catalog: items})
		cancel()
		r.LatencyMs = int(time.Since(start).Milliseconds())
		rep.Provider, rep.Model = out.Provider, out.Model
		rep.TokensIn += out.TokensIn
		rep.TokensOut += out.TokensOut
		rep.CostMicroUSD += out.CostMicroUSD
		if err == nil {
			r.Got, err = domain.ParseUnderstanding(out.Raw, c.Text, catalog)
		}
		if r.Err = err; err != nil {
			rep.Invalid++
			rep.Results = append(rep.Results, r)
			continue
		}
		r.IntentOK = r.Got.Intent == c.Intent || slices.Contains(c.Intents, r.Got.Intent)
		r.VehicleOK = c.Vehicle == "" || r.Got.Vehicle.Type == c.Vehicle
		r.Correct = servicesOK(r.Got.Services, c.Services, c.Acceptable)
		d := domain.BuildDraft(domain.DraftInput{CustomerName: "Cliente", Workshop: s.Workshop.Name, Intent: r.Got.ToIntent(c.Text),
			Services: services, Parts: parts, NextOpening: "mañana a las 8:00 a. m.", HasPlate: r.Got.Plate != "", Attends: s.Workshop.Attends})
		r.Draft, r.OutOfList = d.Text, domain.Validate(d.Text, all) != nil
		count(&rep.Intents, r.IntentOK)
		count(&rep.Vehicles, r.VehicleOK)
		count(&rep.Services, r.Correct)
		count(&rep.OutOfList, r.OutOfList)
		rep.Results = append(rep.Results, r)
	}
	return rep
}

func count(n *int, ok bool) {
	if ok {
		*n++
	}
}

// servicesOK: salen todos los esperados y nada fuera de esperados + aceptables.
func servicesOK(got, want, acceptable []string) bool {
	norm := func(l []string) []string {
		out := make([]string, 0, len(l))
		for _, x := range l {
			out = append(out, domain.Normalize(x))
		}
		return out
	}
	g, w, a := norm(got), norm(want), norm(acceptable)
	for _, x := range w {
		if !slices.Contains(g, x) {
			return false
		}
	}
	for _, x := range g {
		if !slices.Contains(w, x) && !slices.Contains(a, x) {
			return false
		}
	}
	return true
}

// Print escribe el informe en texto para la terminal y para pegar en docs/07 (D-08).
func (r Report) Print(w io.Writer) {
	n := len(r.Results)
	_, _ = fmt.Fprintf(w, "Proveedor %s · modelo %s · %d mensajes\n", r.Provider, r.Model, n)
	for i, x := range r.Results {
		mark := "ok"
		switch {
		case x.Err != nil:
			mark = "INVÁLIDO"
		case !x.Correct || !x.IntentOK || !x.VehicleOK:
			mark = "falla"
		}
		_, _ = fmt.Fprintf(w, "%2d. [%s] %s\n", i+1, mark, x.Text)
		if x.Err != nil {
			_, _ = fmt.Fprintf(w, "      error: %v\n", x.Err)
			continue
		}
		if mark != "ok" {
			_, _ = fmt.Fprintf(w, "      intención %s (esperada %s) · tipo %q (esperado %q) · servicios [%s] (esperados [%s])\n", x.Got.Intent, x.Intent,
				x.Got.Vehicle.Type, x.Vehicle, strings.Join(x.Got.Services, ", "), strings.Join(x.Services, ", "))
		}
	}
	_, _ = fmt.Fprintf(w, "\nServicios bien: %d/%d (HU-102 pide %d) · intención: %d/%d · tipo de vehículo: %d/%d\n",
		r.Services, n, MinServices, r.Intents, n, r.Vehicles, n)
	_, _ = fmt.Fprintf(w, "Salidas inválidas: %d · borradores con precios fuera de lista: %d\n", r.Invalid, r.OutOfList)
	_, _ = fmt.Fprintf(w, "Tokens: %d de entrada, %d de salida · costo: US$%.4f\n", r.TokensIn, r.TokensOut, float64(r.CostMicroUSD)/1e6)
}
