// Command pin-reset pone un PIN temporal a cualquier usuario cuando nadie del taller puede hacerlo: típicamente el
// dueño olvidó el suyo (HU-104, Q-40; en la entrega no hay recuperación por SMS).
//
//	make pin-reset CELULAR=3001234567 PIN_NUEVO=4826
//
// Lo corre un administrador con MIGRATIONS_DATABASE_URL (rol sweveh_migrator, sujeto a RLS). Usa el mismo pepper que
// el api (PIN_PEPPER u OTP_PEPPER). Deja pin_cambiar = true (al entrar escoge el suyo), desbloquea el celular, cierra
// sus sesiones abiertas y anota
// "cuenta.pin_reset" en la bitácora de cada taller del usuario, con actor "sistema" y sin el PIN.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"sweveh/internal/identity/adapters/out/pinhash"
	identitydomain "sweveh/internal/identity/domain"
	"sweveh/internal/platform/config"
	"sweveh/internal/platform/db"
	"sweveh/internal/platform/logging"
	"sweveh/internal/shared"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pin-reset:", err)
		os.Exit(1)
	}
}

func run() error {
	url := os.Getenv("MIGRATIONS_DATABASE_URL")
	if url == "" {
		return errors.New("MIGRATIONS_DATABASE_URL (sweveh_migrator) es obligatoria")
	}
	phone, err := shared.ParseColombianMobile(os.Getenv("CELULAR"))
	if err != nil {
		return errors.New("CELULAR debe ser un celular colombiano de 10 dígitos")
	}
	pin, err := identitydomain.ParsePIN(os.Getenv("PIN_NUEVO"))
	if err != nil {
		return fmt.Errorf("PIN_NUEVO: %w", err)
	}
	pepper, err := config.PINPepperFromEnv()
	if err != nil {
		return err
	}
	hasher, err := pinhash.New([]byte(pepper), pinhash.OWASP)
	if err != nil {
		return err
	}
	hash, err := hasher.Hash(pin)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, db.PoolConfig{URL: url, MaxConns: 2})
	if err != nil {
		return err
	}
	defer pool.Close()
	tx := db.NewTxManager(pool)

	var userID shared.ID
	var talleres []shared.ID
	now := time.Now().UTC()
	err = tx.DoWithSettings(ctx, nil, func(ctx context.Context) error {
		id, err := db.Scalar[shared.ID](ctx, `UPDATE identity.usuario SET pin_hash = $2, pin_cambiar = true, updated_at = $3
			WHERE celular = $1 RETURNING id`, phone, hash, now)
		if err != nil {
			return fmt.Errorf("no hay un usuario con ese celular: %w", err)
		}
		userID = id
		if _, err := db.Exec(ctx, `DELETE FROM identity.otp_bloqueo WHERE celular = $1`, phone); err != nil {
			return err
		}
		// Igual que "Restablecer PIN" en Equipo: quien tuviera una sesión abierta de esa cuenta la pierde.
		if _, err := db.Exec(ctx, `UPDATE identity.sesion SET revocada_en = $2, motivo_revocacion = 'pin_restablecido'
			WHERE usuario_id = $1 AND revocada_en IS NULL`, id, now); err != nil {
			return err
		}
		if err := tx.SetLocal(ctx, "app.usuario_id", id.String()); err != nil {
			return err
		}
		rows, err := db.Many[struct {
			TallerID shared.ID `db:"taller_id"`
		}](ctx, `SELECT taller_id FROM identity.membresia WHERE usuario_id = $1 AND activo`, id)
		for _, r := range rows {
			talleres = append(talleres, r.TallerID)
		}
		return err
	})
	if err != nil {
		return err
	}
	// La bitácora es por taller (RLS): una transacción corta con el taller fijado por cada uno.
	for _, t := range talleres {
		if err := tx.DoWithSettings(ctx, map[string]string{"app.taller_id": t.String()}, func(ctx context.Context) error {
			_, err := db.Exec(ctx, `INSERT INTO audit.evento (id, taller_id, actor_tipo, accion, entidad, entidad_id, ocurrido_en)
				VALUES ($1, $2, 'sistema', 'cuenta.pin_reset', 'usuario', $3, $4)`, shared.NewID(), t, userID, now)
			return err
		}); err != nil {
			return fmt.Errorf("bitácora: %w", err)
		}
	}
	fmt.Printf("Listo: PIN temporal puesto a %s (%d taller(es)). Al entrar tendrá que escoger uno propio.\n", logging.MaskPhone(phone), len(talleres))
	return nil
}
