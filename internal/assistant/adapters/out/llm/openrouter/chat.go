package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	"sweveh/internal/assistant/app"
	"sweveh/internal/assistant/domain"
)

// Chat es el asesor con tools (ADR-0029): la conversación, las tools en formato OpenAI y la respuesta del modelo,
// que puede pedir tools o traer el texto para el cliente. Los guardias del texto los aplica app, no este adaptador.

var _ app.Advisor = (*Client)(nil)

type chatMsg struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type chatRequest struct {
	Model       string     `json:"model"`
	Models      []string   `json:"models,omitempty"`
	Messages    []chatMsg  `json:"messages"`
	Tools       []chatTool `json:"tools,omitempty"`
	ToolChoice  string     `json:"tool_choice,omitempty"`
	Temperature float64    `json:"temperature"`
	MaxTokens   int        `json:"max_tokens"`
	Reasoning   any        `json:"reasoning"`
	Usage       any        `json:"usage"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string         `json:"content"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Code int `json:"code"`
	} `json:"error"`
}

func (c *Client) Chat(ctx context.Context, r app.ChatRequest) (app.ChatResult, error) {
	out := app.ChatResult{Provider: "openrouter"}
	if len(c.models) > 0 {
		out.Model = c.models[0]
	}
	req := chatRequest{Model: out.Model, Temperature: 0.4, MaxTokens: 700, Usage: map[string]bool{"include": true},
		// Igual que al interpretar: con razonamiento los modelos que piensan gastan el límite y responden vacío.
		Reasoning: map[string]bool{"enabled": false}}
	if len(c.models) > 1 {
		req.Models = c.models
	}
	req.Messages = append(req.Messages, chatMsg{Role: "system", Content: r.System})
	for _, m := range r.Messages {
		cm := chatMsg{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		if m.Role == "user" {
			// El texto del cliente va delimitado: es un dato, nunca una instrucción (ver el prompt).
			cm.Content = "<<<\n" + m.Content + "\n>>>"
		}
		for _, tc := range m.ToolCalls {
			call := chatToolCall{ID: tc.ID, Type: "function"}
			call.Function.Name, call.Function.Arguments = tc.Name, tc.Args
			cm.ToolCalls = append(cm.ToolCalls, call)
		}
		req.Messages = append(req.Messages, cm)
	}
	for _, t := range r.Tools {
		ct := chatTool{Type: "function"}
		ct.Function.Name, ct.Function.Description, ct.Function.Parameters = t.Name, t.Description, t.Params
		req.Tools = append(req.Tools, ct)
	}
	if len(req.Tools) > 0 {
		req.ToolChoice = "auto"
	}
	body, err := json.Marshal(req)
	if err != nil {
		return out, err
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return out, err
	}
	hr.Header.Set("Authorization", "Bearer "+c.key)
	hr.Header.Set("Content-Type", "application/json")
	hr.Header.Set("X-Title", "SWEVEH")
	res, err := c.http.Do(hr)
	if err != nil {
		return out, fmt.Errorf("openrouter: %w", redact(err))
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return out, fmt.Errorf("openrouter: %w", err)
	}
	var resp chatResponse
	jerr := json.Unmarshal(raw, &resp)
	if res.StatusCode/100 != 2 {
		e := &APIError{Status: res.StatusCode}
		if jerr == nil && resp.Error != nil {
			e.Code = resp.Error.Code
		}
		return out, e
	}
	if jerr != nil {
		return out, fmt.Errorf("openrouter: respuesta que no es JSON: %w", domain.ErrInvalidOutput)
	}
	out.TokensIn, out.TokensOut = resp.Usage.PromptTokens, resp.Usage.CompletionTokens
	out.CostMicroUSD = int64(math.Round(resp.Usage.Cost * 1e6))
	if resp.Model != "" {
		out.Model = resp.Model
	}
	if resp.Error != nil {
		return out, &APIError{Status: res.StatusCode, Code: resp.Error.Code}
	}
	if len(resp.Choices) == 0 {
		return out, fmt.Errorf("openrouter: sin respuesta: %w", domain.ErrInvalidOutput)
	}
	ch := resp.Choices[0]
	if ch.Error != nil {
		return out, &APIError{Status: res.StatusCode, Code: ch.Error.Code}
	}
	out.Message = app.ChatMessage{Role: "assistant", Content: ch.Message.Content}
	for _, tc := range ch.Message.ToolCalls {
		out.Message.ToolCalls = append(out.Message.ToolCalls, app.ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments})
	}
	if len(out.Message.ToolCalls) == 0 && ch.Message.Content == "" {
		return out, fmt.Errorf("openrouter: respuesta vacía: %w", domain.ErrInvalidOutput)
	}
	return out, nil
}
