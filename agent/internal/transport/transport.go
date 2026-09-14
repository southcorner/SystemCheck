// Package transport is the agent's mTLS HTTPS client to the server.
package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/southcorner/systemcheck/agent/internal/wire"
)

// Client talks to the SystemCheck server.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewEnrollClient returns a client that trusts the given CA (or the system pool
// if caPEM is empty) but presents no client certificate; used only for /v1/enroll.
func NewEnrollClient(baseURL string, caPEM []byte) *Client {
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if len(caPEM) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caPEM)
		tlsCfg.RootCAs = pool
	}
	return &Client{baseURL: baseURL, http: &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}}
}

// NewMTLSClient returns a client presenting the agent's client certificate and
// trusting the enrollment CA.
func NewMTLSClient(baseURL, certPath, keyPath, caPath string) (*Client, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("invalid ca pem")
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}
	return &Client{baseURL: baseURL, http: &http.Client{
		Timeout:   60 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}}, nil
}

// Enroll exchanges a token + CSR for a client certificate.
func (c *Client) Enroll(ctx context.Context, req wire.EnrollRequest) (*wire.EnrollResponse, error) {
	var resp wire.EnrollResponse
	if err := c.postJSON(ctx, "/v1/enroll", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPolicy fetches the current policy.
func (c *Client) GetPolicy(ctx context.Context) (*wire.Policy, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/policy", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("policy: status %d", res.StatusCode)
	}
	var pol wire.Policy
	if err := json.NewDecoder(res.Body).Decode(&pol); err != nil {
		return nil, err
	}
	return &pol, nil
}

// Ingest posts a batch of events.
func (c *Client) Ingest(ctx context.Context, batch wire.IngestBatch) error {
	return c.postJSON(ctx, "/v1/ingest", batch, nil)
}

// Heartbeat posts liveness.
func (c *Client) Heartbeat(ctx context.Context, hb wire.Heartbeat) error {
	return c.postJSON(ctx, "/v1/heartbeat", hb, nil)
}

// UploadScreenshot posts a screenshot blob and returns the stored object key.
func (c *Client) UploadScreenshot(ctx context.Context, objectKey string, data []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("object_key", objectKey)
	fw, err := mw.CreateFormFile("file", objectKey)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(data); err != nil {
		return "", err
	}
	mw.Close()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/screenshots", &buf)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("upload: status %d: %s", res.StatusCode, string(b))
	}
	var out struct {
		ObjectKey string `json:"object_key"`
	}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return out.ObjectKey, nil
}

func (c *Client) postJSON(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%s: status %d: %s", path, res.StatusCode, string(rb))
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}
