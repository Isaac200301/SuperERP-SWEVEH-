package domain

import (
	"regexp"
	"strings"

	"sweveh/internal/shared"
)

// ChecklistItem es algo que se revisa al recibir el vehículo (luces, gato, documentos…). La clave es estable para que
// los checklists ya hechos sigan legibles aunque el taller renombre el ítem.
type ChecklistItem struct {
	Key  string `json:"clave"`
	Name string `json:"nombre"`
}

// Parametros son las reglas de trabajo que cada taller ajusta (decisión de Pablo, 3-nov-2026). Lo que ya se
// cotizó o recibió no cambia si el taller cambia un parámetro: la cotización y el checklist guardan su copia.
type Parametros struct {
	UseAwaitingParts bool // estado opcional "Esperando repuesto"
	UseQualityCheck  bool // estado opcional "Control de calidad"
	MinPhotos        int
	MileageRequired  bool
	CarItems         []ChecklistItem
	MotoItems        []ChecklistItem
	VATPct           int  // IVA; por defecto 19, a confirmar con un contador (docs/07)
	PricesIncludeVAT bool // los precios de lista ya traen IVA
	HourlyRate       *int64
	MaxDiscountPct   int
	ReceptionDiscount bool // recepción puede dar descuentos (el dueño siempre puede)
	ValidityDays     int
	Version          int
}

// Precarga sugerida de la recepción (provisional, Q-02): el taller la edita en Mi taller → Parámetros.
var (
	DefaultCarItems = []ChecklistItem{{"luces", "Luces"}, {"testigos", "Testigos del tablero"}, {"llanta_repuesto", "Llanta de repuesto"},
		{"gato_herramienta", "Gato y herramienta"}, {"radio", "Radio"}, {"tapetes", "Tapetes"}, {"documentos", "Documentos"}}
	DefaultMotoItems = []ChecklistItem{{"luces", "Luces"}, {"testigos", "Testigos del tablero"}, {"espejos", "Espejos"},
		{"casco", "Casco"}, {"baul", "Baúl"}, {"documentos", "Documentos"}}
)

// DefaultParametros son los valores con los que arranca un taller que no ha configurado nada.
func DefaultParametros() Parametros {
	return Parametros{MinPhotos: 4, MileageRequired: true, CarItems: DefaultCarItems, MotoItems: DefaultMotoItems,
		VATPct: 19, PricesIncludeVAT: true, MaxDiscountPct: 10, ValidityDays: 15}
}

var (
	ErrParamPhotos   = shared.Validation("parametro_fotos", "El mínimo de fotos va de 0 a 8.").WithField("fotosMinimas")
	ErrParamVAT      = shared.Validation("parametro_iva", "El IVA va de 0 % a 30 %.").WithField("ivaPct")
	ErrParamRate     = shared.Validation("parametro_tarifa", "La tarifa por hora va en pesos y no puede ser negativa.").WithField("tarifaHora")
	ErrParamDiscount = shared.Validation("parametro_descuento", "El descuento máximo va de 0 % a 100 %.").WithField("descuentoMaxPct")
	ErrParamValidity = shared.Validation("parametro_vigencia", "La vigencia de la cotización va de 1 a 90 días.").WithField("vigenciaDias")
	ErrParamItems    = shared.Validation("parametro_items", "Cada ítem de la recepción necesita un nombre de hasta 60 letras; máximo 30 ítems sin repetir.").WithField("items")

	reKey = regexp.MustCompile(`[^a-z0-9]+`)
)

// Validate revisa rangos y limpia la lista de ítems (nombres sin espacios de más, claves estables y únicas).
func (p *Parametros) Validate() error {
	switch {
	case p.MinPhotos < 0 || p.MinPhotos > 8:
		return ErrParamPhotos
	case p.VATPct < 0 || p.VATPct > 30:
		return ErrParamVAT
	case p.HourlyRate != nil && *p.HourlyRate < 0:
		return ErrParamRate
	case p.MaxDiscountPct < 0 || p.MaxDiscountPct > 100:
		return ErrParamDiscount
	case p.ValidityDays < 1 || p.ValidityDays > 90:
		return ErrParamValidity
	}
	var err error
	if p.CarItems, err = cleanItems(p.CarItems); err != nil {
		return err
	}
	p.MotoItems, err = cleanItems(p.MotoItems)
	return err
}

func cleanItems(in []ChecklistItem) ([]ChecklistItem, error) {
	if len(in) > 30 {
		return nil, ErrParamItems
	}
	out := make([]ChecklistItem, 0, len(in))
	seen := map[string]bool{}
	for _, it := range in {
		it.Name = strings.Join(strings.Fields(it.Name), " ")
		if it.Name == "" || len([]rune(it.Name)) > 60 {
			return nil, ErrParamItems
		}
		if it.Key == "" {
			it.Key = KeyFor(it.Name)
		}
		if seen[it.Key] {
			return nil, ErrParamItems
		}
		seen[it.Key] = true
		out = append(out, it)
	}
	return out, nil
}

// KeyFor arma una clave estable desde el nombre: "Llanta de repuesto" → "llanta_de_repuesto".
func KeyFor(name string) string {
	s := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u").Replace(strings.ToLower(name))
	return strings.Trim(reKey.ReplaceAllString(s, "_"), "_")
}

// ItemsFor devuelve los ítems de recepción del tipo de vehículo.
func (p Parametros) ItemsFor(kind string) []ChecklistItem {
	if kind == "moto" {
		return p.MotoItems
	}
	return p.CarItems
}
