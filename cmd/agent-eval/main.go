// Command agent-eval pasa el juego de 30 mensajes (HU-102) por el intérprete configurado y dice si cumple:
// ningún precio fuera de lista y al menos 25 servicios bien sacados. Sirve para elegir modelo (D-08).
//
//	AI_PROVIDER=openrouter AI_API_KEY=... AI_MODEL=openai/gpt-4o-mini go run ./cmd/agent-eval
//
// Sin variables usa las reglas locales. No toca la base de datos ni datos de clientes.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"sweveh/internal/assistant/eval"
	"sweveh/internal/platform/config"
	"sweveh/internal/wire"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "agent-eval:", err)
		os.Exit(1)
	}
}

func run() error {
	ai, err := config.LoadAI()
	if err != nil {
		return err
	}
	set, err := eval.Default()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	rep := eval.Run(ctx, wire.Interpreter(ai), set)
	rep.Print(os.Stdout)
	if !rep.Passes() {
		return fmt.Errorf("no cumple HU-102: %d/%d servicios bien (mínimo %d), %d con precios fuera de lista",
			rep.Services, len(rep.Results), eval.MinServices, rep.OutOfList)
	}
	fmt.Println("Cumple HU-102.")
	return nil
}
