// Package groq transcribe las notas de voz con la API de Groq (Whisper, compatible con OpenAI). Solo para la demo
// hasta cerrar AQ-09: config no lo deja arrancar en producción (ADR-0033).
// Referencia (consultada el 2026-11-05): https://console.groq.com/docs/speech-to-text
//
//	POST {base}/audio/transcriptions, multipart: file, model, language, response_format=json → {"text": "..."}
//
// Acepta ogg (las notas de voz de WhatsApp son ogg/opus), mp3, m4a, wav y webm.
package groq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Client llama a Groq con su clave. La clave y el audio nunca salen en errores ni en logs.
type Client struct {
	base, key, model string
	http             *http.Client
}

// New crea el cliente. base es p. ej. "https://api.groq.com/openai/v1" (STT_API_BASE).
func New(base, key, model string, timeout time.Duration) *Client {
	return &Client{base: strings.TrimRight(base, "/"), key: key, model: model, http: &http.Client{Timeout: timeout}}
}

// APIError es una respuesta no exitosa: solo el código HTTP.
type APIError struct{ Status int }

func (e *APIError) Error() string { return fmt.Sprintf("groq: respuesta %d", e.Status) }

// extensions: Groq reconoce el formato por el nombre del archivo.
var extensions = map[string]string{"audio/ogg": "ogg", "audio/opus": "ogg", "audio/mpeg": "mp3", "audio/mp4": "m4a",
	"audio/aac": "m4a", "audio/wav": "wav", "audio/x-wav": "wav", "audio/webm": "webm", "audio/amr": "amr"}

// Transcribe devuelve el texto de la nota de voz, en español.
func (c *Client) Transcribe(ctx context.Context, audio []byte, mime string) (string, error) {
	if len(audio) == 0 {
		return "", errors.New("groq: audio vacío")
	}
	ext, ok := extensions[strings.TrimSpace(strings.Split(mime, ";")[0])]
	if !ok {
		ext = "ogg"
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "nota."+ext)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(audio); err != nil {
		return "", err
	}
	for k, v := range map[string]string{"model": c.model, "language": "es", "response_format": "json", "temperature": "0"} {
		if err := w.WriteField(k, v); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", &APIError{Status: res.StatusCode}
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", errors.New("groq: respuesta que no es JSON")
	}
	return strings.TrimSpace(out.Text), nil
}
