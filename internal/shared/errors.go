package shared

import "errors"

// Kind clasifica un error para que el borde HTTP elija el código (RFC 9457).
type Kind int

const (
	KindUnknown      Kind = iota
	KindValidation        // 400: la entrada no tiene el formato esperado
	KindUnauthorized      // 401
	KindForbidden         // 403
	KindNotFound          // 404: también para recursos de otro taller
	KindConflict          // 409: versión o estado en conflicto
	KindRule              // 422: la entrada es válida pero una regla de negocio la impide
	KindRateLimited       // 429
)

// Error es un error de dominio tipado. Code es estable (lo lee el frontend);
// Message es el texto en español de Colombia para la persona.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Field   string // campo del formulario al que se refiere, si aplica
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Is compara por código, para que errors.Is funcione con copias que llevan otro campo.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// WithField devuelve una copia del error ligada a un campo del formulario.
func (e *Error) WithField(field string) *Error {
	c := *e
	c.Field = field
	return &c
}

// WithMessage devuelve una copia del error con otro texto para la persona.
func (e *Error) WithMessage(msg string) *Error {
	c := *e
	c.Message = msg
	return &c
}

func newErr(k Kind, code, msg string) *Error { return &Error{Kind: k, Code: code, Message: msg} }

func Validation(code, msg string) *Error   { return newErr(KindValidation, code, msg) }
func Unauthorized(code, msg string) *Error { return newErr(KindUnauthorized, code, msg) }
func Forbidden(code, msg string) *Error    { return newErr(KindForbidden, code, msg) }
func NotFound(code, msg string) *Error     { return newErr(KindNotFound, code, msg) }
func Conflict(code, msg string) *Error     { return newErr(KindConflict, code, msg) }
func Rule(code, msg string) *Error         { return newErr(KindRule, code, msg) }
func RateLimited(code, msg string) *Error  { return newErr(KindRateLimited, code, msg) }

// ErrVersionConflict: otra persona cambió el registro (concurrencia optimista, R-07).
var ErrVersionConflict = Conflict("version_conflicto", "Otra persona cambió este registro. Recarga y vuelve a intentar.")

// AsError extrae el *Error de una cadena de errores, si existe.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
