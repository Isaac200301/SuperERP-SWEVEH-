// Package domain contiene las reglas del cliente del taller (HU-09, RF-04). Sin I/O.
package domain

import (
	"regexp"
	"strings"
	"time"

	"sweveh/internal/shared"
)

// ConsentStatus es el estado de la autorización de datos del cliente (Ley 1581, HU-85/86).
type ConsentStatus string

const (
	ConsentPending  ConsentStatus = "pendiente"
	ConsentGranted  ConsentStatus = "autorizada"
	ConsentRejected ConsentStatus = "rechazada"
)

// Texto y versión de la autorización del cliente final en el mostrador (HU-86). Borrador hasta la revisión jurídica
// (AQ-10): no lo redacta el agente; cambiarlo es cambiar la versión.
const (
	CounterConsentVersion = "autorizacion-cliente-borrador-2026-11-01"
	CounterConsentText    = "Autorizo al taller a tratar mis datos (nombre, celular y datos del vehículo) para atender mi vehículo y avisarme por WhatsApp."
)

var (
	ErrCustomerNotFound = shared.NotFound("cliente_no_encontrado", "No encontramos ese cliente en tu taller.")
	ErrNameRequired     = shared.Validation("nombre_requerido", "Escribe el nombre del cliente.").WithField("nombre")
	ErrNameTooLong      = shared.Validation("nombre_largo", "El nombre puede tener hasta 120 letras.").WithField("nombre")
	ErrEmailInvalid     = shared.Validation("correo_invalido", "Revisa el correo, por ejemplo nombre@correo.com.").WithField("correo")
	// ErrConsentRequired (HU-86, R-10, R11): un cliente nuevo del mostrador no se guarda sin su respuesta a la
	// autorización de datos. Puede decir que no (la orden sigue, HU-86 criterio 2); el silencio no cuenta.
	ErrConsentRequired = shared.Validation("autorizacion_requerida",
		"Antes de guardar al cliente, pregúntale si autoriza el tratamiento de sus datos y marca su respuesta.").WithField("autorizacion")
)

var reEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Customer es el agregado cliente.
type Customer struct {
	ID        shared.ID
	TallerID  shared.ID
	Phone     string // E.164
	Name      string
	Email     *string
	Consent   ConsentStatus
	Version   int
	CreatedAt time.Time
}

// IsPlaceholderName dice si el nombre es de relleno: vacío, de una letra, solo números o el que se pone cuando
// WhatsApp no trae nombre ("Cliente WhatsApp", "Cliente de prueba"). Solo esos se reemplazan con el nombre que da el
// cliente al asesor; un nombre real no se toca.
func IsPlaceholderName(name string) bool {
	f := strings.Fields(name)
	if len(f) == 0 {
		return true
	}
	first := strings.ToLower(f[0])
	return len([]rune(f[0])) < 2 || first == "cliente" || strings.Trim(f[0], "0123456789+") == ""
}

// CleanName limpia y valida un nombre de cliente.
func CleanName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	if name == "" {
		return "", ErrNameRequired
	}
	if len([]rune(name)) > 120 {
		return "", ErrNameTooLong
	}
	return name, nil
}

// NewCustomer valida y crea un cliente con autorización pendiente (HU-09, criterio 3).
func NewCustomer(id, tallerID shared.ID, rawPhone, rawName string, rawEmail *string, now time.Time) (Customer, error) {
	phone, err := shared.ParseColombianMobile(rawPhone)
	if err != nil {
		return Customer{}, shared.ErrPhoneInvalid.WithField("celular")
	}
	name := strings.Join(strings.Fields(rawName), " ")
	if name == "" {
		return Customer{}, ErrNameRequired
	}
	if len([]rune(name)) > 120 {
		return Customer{}, ErrNameTooLong
	}
	var email *string
	if rawEmail != nil {
		e := strings.ToLower(strings.TrimSpace(*rawEmail))
		if e != "" {
			if !reEmail.MatchString(e) || len(e) > 254 {
				return Customer{}, ErrEmailInvalid
			}
			email = &e
		}
	}
	return Customer{
		ID:        id,
		TallerID:  tallerID,
		Phone:     phone,
		Name:      name,
		Email:     email,
		Consent:   ConsentPending,
		Version:   1,
		CreatedAt: now,
	}, nil
}

// NameKey deja el nombre comparable para encontrar repetidos (HU-15): sin tildes, en minúscula y con espacios simples.
// Debe coincidir con la clave de Duplicates en el repositorio.
func NameKey(name string) string {
	r := strings.NewReplacer("Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N",
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
	return strings.ToLower(strings.Join(strings.Fields(r.Replace(name)), " "))
}
