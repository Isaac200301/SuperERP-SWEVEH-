package wire

import (
	"context"
	"fmt"

	"sweveh/internal/messaging/adapters/out/whatsapp/waha"
	"sweveh/internal/platform/config"
)

// WAHAOTP entrega el código de entrada por el WhatsApp de prueba (WAHA, ADR-0034): solo demo, con números de prueba.
// Vive aquí y no en identity porque junta dos módulos: el puerto OTPSender es de identity y el cliente, de messaging.
// El código nunca se escribe en logs ni en errores (docs/08 §6): el cliente de WAHA solo devuelve el estado HTTP.
type WAHAOTP struct {
	client  *waha.Client
	session string
}

// NewWAHAOTP arma el envío con la sesión de WAHA_SESSION.
func NewWAHAOTP(w config.WAHA) *WAHAOTP { return &WAHAOTP{client: WAHAClient(w), session: w.Session} }

func (o *WAHAOTP) Send(ctx context.Context, phone, code string) error {
	text := fmt.Sprintf("Tu código de SWEVEH es *%s*. Vence en 10 minutos. No lo compartas con nadie.", code)
	_, err := o.client.SendText(ctx, waha.Prefix+o.session, phone, text)
	return err
}
