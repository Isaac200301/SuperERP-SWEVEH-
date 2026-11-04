package app

import (
	"context"
	"time"

	"sweveh/internal/assistant/domain"
	"sweveh/internal/shared"
)

// El asesor (ADR-0029): conversa con el cliente usando tools, con el historial de la conversación. Solo actúa con el
// envío automático encendido (ADR-0028); apagado, sigue el flujo de borradores para revisión. Si la IA falla, se
// agota o su texto no pasa los guardias, responde el flujo de reglas: el cliente nunca queda sin respuesta.

// Advisor es el puerto del modelo con tools (docs/06 §5, LLMProvider).
type Advisor interface {
	Chat(ctx context.Context, r ChatRequest) (ChatResult, error)
}

type ChatRequest struct {
	System   string
	Messages []ChatMessage
	Tools    []ToolSpec
}

// ChatMessage es un turno: role user | assistant | tool. Un assistant puede pedir tools (ToolCalls); la respuesta
// de cada tool va como role tool con su ToolCallID.
type ChatMessage struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type ToolCall struct{ ID, Name, Args string }

// ToolSpec describe una tool: Params es el JSON Schema de sus argumentos.
type ToolSpec struct {
	Name, Description string
	Params            []byte
}

type ChatResult struct {
	Message             ChatMessage
	Provider, Model     string
	TokensIn, TokensOut int
	CostMicroUSD        int64
}

// Turn es un mensaje de la conversación para el historial del asesor.
type Turn struct {
	FromCustomer bool
	Text         string
	At           time.Time
}

const (
	maxRounds      = 4  // rondas de tools por mensaje
	historyTurns   = 12 // mensajes anteriores que ve el asesor
	advisorTimeout = 45 * time.Second
)

// SetAdvisor conecta el asesor con tools (composition root). Sin asesor queda el flujo de reglas.
func (s *Service) SetAdvisor(a Advisor) { s.advisor = a }

// advise responde con el asesor. handled=false: no aplica o falló sin efectos, y el llamador sigue con las reglas;
// tried=true si llamó a la IA (entonces el respaldo usa las reglas sin costo, sin volver a llamarla).
func (s *Service) advise(ctx context.Context, tallerID shared.ID, in Inbound, c Customer) (handled, tried bool, err error) {
	if s.advisor == nil || in.Kind != "texto" {
		return false, false, nil
	}
	var on bool
	var spent int64
	if err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		if on, err = s.repo.AutoReply(ctx, tallerID); err != nil {
			return err
		}
		spent, err = s.repo.MonthCost(ctx, tallerID, monthStart(s.clock.Now()))
		return err
	}); err != nil {
		return false, false, err
	}
	if !on || spent >= s.capUSD {
		return false, false, nil
	}
	actx, err := s.advisorContext(ctx, tallerID, c)
	if err != nil {
		return false, false, err
	}
	history, err := s.taller.History(ctx, tallerID, in.ConversationID, in.At, historyTurns)
	if err != nil {
		return false, false, err
	}
	msgs := make([]ChatMessage, 0, len(history)+1)
	for _, h := range history {
		role := "assistant"
		if h.FromCustomer {
			role = "user"
		}
		msgs = append(msgs, ChatMessage{Role: role, Content: h.Text})
	}
	msgs = append(msgs, ChatMessage{Role: "user", Content: in.Text})

	t := &turn{s: s, tallerID: tallerID, in: in, c: c, quoted: map[int64]bool{}, products: map[string]Product{}}
	cctx, cancel := context.WithTimeout(ctx, advisorTimeout)
	defer cancel()
	req := ChatRequest{System: actx.System(), Messages: msgs, Tools: AdvisorTools}
	for round := 0; round < maxRounds; round++ {
		if round == maxRounds-1 {
			req.Tools = nil // última ronda: tiene que responder
		}
		res, err := s.chat(cctx, tallerID, in, req)
		if err != nil {
			return t.handed, true, s.afterHandoff(ctx, tallerID, in, t, err)
		}
		if len(res.ToolCalls) == 0 {
			return true, true, s.reply(ctx, tallerID, in, t, res.Content)
		}
		req.Messages = append(req.Messages, res)
		for _, call := range res.ToolCalls {
			req.Messages = append(req.Messages, ChatMessage{Role: "tool", ToolCallID: call.ID, Content: t.run(cctx, call.Name, call.Args)})
		}
	}
	return t.handed, true, s.afterHandoff(ctx, tallerID, in, t, nil)
}

// afterHandoff: la IA falló o no cerró. Si ya pasó la conversación a una persona, se avisa al cliente con el texto fijo;
// si no, el llamador responde con las reglas.
func (s *Service) afterHandoff(ctx context.Context, tallerID shared.ID, in Inbound, t *turn, cause error) error {
	if !t.handed {
		return nil
	}
	_, err := s.taller.Send(ctx, tallerID, in.Phone, humanText, nil, "AU-04", nil)
	if err == nil && cause != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// reply aplica los guardias y envía. Si el texto no pasa, sale la respuesta segura de las reglas.
func (s *Service) reply(ctx context.Context, tallerID shared.ID, in Inbound, t *turn, text string) error {
	if err := domain.Guard(text, t.quoted); err != nil {
		if t.handed {
			_, err := s.taller.Send(ctx, tallerID, in.Phone, humanText, nil, "AU-04", nil)
			return err
		}
		return s.saveDraft(ctx, tallerID, in, "asesor", cantAnswer, nil, nil, "plantilla")
	}
	if err := s.answer(ctx, tallerID, in, "asesor", text, nil, nil, "ia"); err != nil {
		return err
	}
	for _, p := range t.photos {
		if _, err := s.taller.SendMedia(ctx, tallerID, in.Phone, p.caption, p.ref); err != nil {
			return err
		}
	}
	return nil
}

// chat hace una llamada al modelo y la registra (HU-99), ligada al mensaje.
func (s *Service) chat(ctx context.Context, tallerID shared.ID, in Inbound, req ChatRequest) (ChatMessage, error) {
	start := time.Now()
	r, err := s.advisor.Chat(ctx, req)
	result := "ok"
	if err != nil {
		result = "error"
	}
	if r.Provider == "" {
		r.Provider = "desconocido"
	}
	if r.Model == "" {
		r.Model = "desconocido"
	}
	id := in.ID
	call := AICall{MessageID: &id, Provider: r.Provider, Model: r.Model, Purpose: "asesorar", Result: result, TokensIn: r.TokensIn,
		TokensOut: r.TokensOut, LatencyMs: int(time.Since(start).Milliseconds()), CostMicroUSD: r.CostMicroUSD}
	if lerr := s.uow.Do(context.WithoutCancel(ctx), func(ctx context.Context) error { return s.repo.LogCall(ctx, tallerID, call) }); lerr != nil {
		return ChatMessage{}, lerr
	}
	return r.Message, err
}

func (s *Service) advisorContext(ctx context.Context, tallerID shared.ID, c Customer) (AdvisorContext, error) {
	prof, err := s.taller.Profile(ctx, tallerID)
	if err != nil {
		return AdvisorContext{}, err
	}
	_, open, next, err := s.taller.Workshop(ctx, tallerID)
	if err != nil {
		return AdvisorContext{}, err
	}
	vs, err := s.taller.Vehicles(ctx, tallerID, c.ID)
	return AdvisorContext{Workshop: prof, Open: open, NextOpening: next, Customer: c, Vehicles: vs}, err
}
