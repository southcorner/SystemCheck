// Package notify forwards alerts to an external endpoint (SIEM/webhook). It is
// best-effort: failures are logged, never fatal to ingestion.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Webhook posts alert payloads as JSON to a configured URL.
type Webhook struct {
	url  string
	http *http.Client
}

// NewWebhook returns a Webhook, or nil if url is empty (notifications disabled).
func NewWebhook(url string) *Webhook {
	if url == "" {
		return nil
	}
	return &Webhook{url: url, http: &http.Client{Timeout: 10 * time.Second}}
}

// AlertPayload is the JSON forwarded per alert.
type AlertPayload struct {
	Source    string                 `json:"source"`
	MachineID string                 `json:"machine_id"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// Send posts one alert. A nil Webhook is a no-op, so callers need not nil-check.
func (w *Webhook) Send(ctx context.Context, p AlertPayload) error {
	if w == nil {
		return nil
	}
	p.Source = "systemcheck"
	if p.Timestamp.IsZero() {
		p.Timestamp = time.Now().UTC()
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
