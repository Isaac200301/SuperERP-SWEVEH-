// Command agent-eval pasa el juego de 30 mensajes (HU-102) por el intérprete configurado y dice si cumple:
// ningún precio fuera de lista y al menos 25 servicios bien sacados. Sirve para elegir modelo (D-08).
//
//	AI_PROVIDER=openrouter AI_API_KEY=... AI_MODEL=openai/gpt-4o-mini go run ./cmd/agent-eval
//
// Sin variables usa las reglas locales. No toca la base de datos ni datos de clientes.
//
// Con -conversaciones mide el asesor con tools (ADR-0029) con el juego de conversaciones; necesita AI_PROVIDER=openrouter.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"sweveh/internal/assistant/adapters/out/llm/openrouter"
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
	conv := flag.Bool("conversaciones", false, "mide el asesor con tools con el juego de conversaciones (ADR-0029)")
	flag.Parse()
	ai, err := config.LoadAI()
	if err != nil {
		return err
	}
	if *conv {
		return conversations(ai)
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

func conversations(ai config.AI) error {
	if ai.Provider != "openrouter" {
		return errors.New("el asesor necesita AI_PROVIDER=openrouter con AI_API_KEY y AI_MODEL")
	}
	set, err := eval.DefaultConversations()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	rep := eval.RunConversations(ctx, openrouter.New(ai.APIBase, ai.APIKey, ai.Model, 30*time.Second), set)
	rep.Print(os.Stdout)
	if !rep.Passes() {
		return fmt.Errorf("no cumple: %d/%d conversaciones (mínimo %d), %d precios no consultados",
			rep.Passed, rep.Conversations, eval.MinConversations, rep.GuardFails)
	}
	fmt.Println("Cumple ADR-0029.")
	return nil
}
