package httpx

import "encoding/json"

// Optional distingue un campo ausente de uno enviado en null en un PATCH.
type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// Ptr devuelve nil si el campo no vino; si vino, un puntero al valor (que puede ser nil).
func (o Optional[T]) Ptr() **T {
	if !o.Set {
		return nil
	}
	v := o.Value
	return &v
}
