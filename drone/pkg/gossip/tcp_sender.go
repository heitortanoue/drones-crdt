package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPTCPSender implementa TCPSender usando HTTP client
type HTTPTCPSender struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPTCPSender cria um novo sender TCP via HTTP
func NewHTTPTCPSender(timeout time.Duration) *HTTPTCPSender {
	return &HTTPTCPSender{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// SendDelta envia delta para URL via POST /delta
func (hts *HTTPTCPSender) SendDelta(url string, delta DeltaMsg) error {
	// Prepara payload JSON
	payload, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("erro ao serializar delta: %v", err)
	}

	// Monta URL completa
	fullURL := fmt.Sprintf("%s/delta", url)

	// Cria request
	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("erro ao criar request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "drone-gossip/1.0")

	// Envia request
	resp, err := hts.client.Do(req)
	if err != nil {
		return fmt.Errorf("erro ao enviar request: %v", err)
	}
	defer resp.Body.Close()

	// Verifica status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status HTTP %d ao enviar delta", resp.StatusCode)
	}

	return nil
}
