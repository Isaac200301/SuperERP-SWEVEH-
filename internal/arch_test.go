package internal_test

// Test de arquitectura (docs/06 §4.1, ADR-01): las reglas de dependencia se hacen cumplir
// con una prueba, no con disciplina. Si falla, se corrige el diseño; nunca se relaja la regla (R-11).

import (
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

const mod = "sweveh/"

// Módulos de negocio con forma hexagonal y transversales que otros pueden usar por su fachada.
var businessModules = []string{"identity", "tenancy", "customers", "vehicles", "orders"}
var crossCutting = []string{"audit", "outbox"}

type pkg struct {
	ImportPath string
	Imports    []string
}

func listPackages(t *testing.T) []pkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "sweveh/...")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var pkgs []pkg
	for {
		var p pkg
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("go list json: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	if len(pkgs) < 20 {
		t.Fatalf("solo se listaron %d paquetes", len(pkgs))
	}
	return pkgs
}

// moduleOf devuelve el módulo y la capa: sweveh/internal/orders/app → ("orders", "app").
func moduleOf(path string) (module, layer string) {
	rest, ok := strings.CutPrefix(path, mod+"internal/")
	if !ok {
		return "", ""
	}
	parts := strings.Split(rest, "/")
	module = parts[0]
	if len(parts) > 1 {
		layer = parts[1]
	}
	return module, layer
}

func isStdlib(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")
	return !strings.Contains(first, ".") && !strings.HasPrefix(imp, mod)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestDependencyRules(t *testing.T) {
	for _, p := range listPackages(t) {
		m, layer := moduleOf(p.ImportPath)
		for _, imp := range p.Imports {
			im, ilayer := moduleOf(imp)
			switch {
			// 1. shared: solo biblioteca estándar y uuid.
			case m == "shared":
				if !isStdlib(imp) && imp != "github.com/google/uuid" {
					t.Errorf("%s (shared) importa %s", p.ImportPath, imp)
				}
			// 2. domain: solo biblioteca estándar, shared y su propio domain.
			case layer == "domain":
				if !isStdlib(imp) && imp != mod+"internal/shared" && (im != m || ilayer != "domain") {
					t.Errorf("%s (domain) importa %s", p.ImportPath, imp)
				}
			// 3. app y ports: nada de adaptadores, HTTP ni drivers de base de datos.
			case layer == "app" || layer == "ports":
				if ilayer == "adapters" || strings.HasPrefix(imp, "github.com/jackc/pgx") || imp == "net/http" ||
					strings.HasPrefix(imp, "github.com/go-chi") || strings.HasPrefix(imp, mod+"internal/platform/") {
					t.Errorf("%s (%s) importa %s", p.ImportPath, layer, imp)
				}
			// 4. platform no conoce los módulos de negocio.
			case m == "platform":
				if contains(businessModules, im) || contains(crossCutting, im) || im == "server" {
					t.Errorf("%s (platform) importa el módulo %s", p.ImportPath, imp)
				}
			}
			// 5. Entre módulos, solo la fachada (raíz del módulo). El composition root (server y wire,
			// que adapta los puertos de un módulo a los de otro) está exento, y también demo: la carga del taller
			// ficticio arma el sistema completo y crea el taller como el registro (HU-97, ADR-0031).
			if m != "" && m != "server" && m != "wire" && m != "demo" && im != "" && im != m && (contains(businessModules, im) || contains(crossCutting, im)) {
				if ilayer != "" {
					t.Errorf("%s importa el interior de otro módulo: %s (solo se permite su fachada)", p.ImportPath, imp)
				}
			}
		}
	}
}

// Los adaptadores de entrada de un módulo nunca importan adaptadores de salida de otro módulo.
func TestAdaptersStayInTheirModule(t *testing.T) {
	for _, p := range listPackages(t) {
		m, layer := moduleOf(p.ImportPath)
		if layer != "adapters" {
			continue
		}
		for _, imp := range p.Imports {
			if im, ilayer := moduleOf(imp); im != m && ilayer == "adapters" {
				t.Errorf("%s importa adaptadores de %s", p.ImportPath, im)
			}
		}
	}
}

// HU-101 (RNF-07): pasar del número de prueba al del taller solo cambia la configuración del canal. Los proveedores
// de WhatsApp (simulador, WAHA, Meta) solo los conocen el módulo messaging y la raíz de composición (wire); el resto
// del sistema habla con el puerto de canal.
func TestWhatsAppProviderBehindChannel(t *testing.T) {
	const providers = mod + "internal/messaging/adapters/out/whatsapp"
	for _, p := range listPackages(t) {
		if strings.HasPrefix(p.ImportPath, mod+"internal/messaging/") || strings.HasPrefix(p.ImportPath, mod+"internal/wire") {
			continue
		}
		for _, imp := range p.Imports {
			if strings.HasPrefix(imp, providers) {
				t.Errorf("%s conoce el proveedor de WhatsApp %s (debe usar el canal de messaging)", p.ImportPath, imp)
			}
		}
	}
}
