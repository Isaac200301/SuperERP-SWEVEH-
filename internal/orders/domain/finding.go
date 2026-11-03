package domain

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"sweveh/internal/shared"
)

// Finding es un hallazgo del diagnóstico (HU-103 propuesta): lo registra una persona del taller, no la IA.
type Finding struct {
	ID          shared.ID
	TallerID    shared.ID
	OrderID     shared.ID
	System      string
	Description string
	Severity    string
	FaultCodes  []string
	AuthorID    *shared.ID
	CreatedAt   time.Time
}

var (
	Systems    = []string{"motor", "frenos", "suspension", "direccion", "electrico", "transmision", "refrigeracion", "llantas", "carroceria", "otro"}
	Severities = []string{"urgente", "recomendado", "puede_esperar"}

	// Códigos OBD-II: P (motor y caja), B (carrocería), C (chasis), U (red), p. ej. P0301.
	reFaultCode = regexp.MustCompile(`^[PBCU][0-3][0-9A-F]{3}$`)

	ErrFindingSystem   = shared.Validation("hallazgo_sistema", "Escoge el sistema del vehículo donde está el hallazgo.").WithField("sistema")
	ErrFindingText     = shared.Validation("hallazgo_descripcion", "Describe el hallazgo en hasta 1.000 letras.").WithField("descripcion")
	ErrFindingSeverity = shared.Validation("hallazgo_gravedad", "La gravedad es urgente, recomendado o puede esperar.").WithField("gravedad")
	ErrFaultCode       = shared.Validation("codigo_falla_invalido", "Los códigos de falla son como P0301: una letra (P, B, C o U) y 4 caracteres.").WithField("codigosFalla")
	ErrFindingState    = shared.Rule("hallazgo_fuera_de_estado", "Los hallazgos se registran con el vehículo en el taller: desde Recibido hasta En reparación.")
	ErrFindingLocked   = shared.Rule("hallazgo_cotizado", "Ya se envió la cotización: el hallazgo no se puede borrar. Agrega uno nuevo si cambió algo.")
	ErrFindingNotFound = shared.NotFound("hallazgo_no_encontrado", "No encontramos ese hallazgo en la orden.")
)

// NewFinding valida y normaliza un hallazgo (códigos en mayúscula, sin repetir, máximo 10).
func NewFinding(id shared.ID, o Order, system, description, severity string, codes []string, author *shared.ID, now time.Time) (Finding, error) {
	if !o.CanAddFindings() {
		return Finding{}, ErrFindingState
	}
	if !slices.Contains(Systems, system) {
		return Finding{}, ErrFindingSystem
	}
	description = strings.Join(strings.Fields(description), " ")
	if description == "" || len([]rune(description)) > 1000 {
		return Finding{}, ErrFindingText
	}
	if !slices.Contains(Severities, severity) {
		return Finding{}, ErrFindingSeverity
	}
	clean := []string{}
	for _, c := range codes {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || slices.Contains(clean, c) {
			continue
		}
		if !reFaultCode.MatchString(c) || len(clean) == 10 {
			return Finding{}, ErrFaultCode
		}
		clean = append(clean, c)
	}
	return Finding{ID: id, TallerID: o.TallerID, OrderID: o.ID, System: system, Description: description, Severity: severity,
		FaultCodes: clean, AuthorID: author, CreatedAt: now}, nil
}

// CanAddFindings: con el vehículo en el taller y la orden abierta. En reparación sirven para la cotización adicional (HU-31).
func (o Order) CanAddFindings() bool {
	return o.State == StateReceived || o.State == StateDiagnosis || o.State == StateRepair
}

// CanRemoveFindings: antes de enviar la cotización; después, el hallazgo ya es parte de lo que vio el cliente.
func (o Order) CanRemoveFindings() bool {
	return o.State == StateReceived || o.State == StateDiagnosis
}
