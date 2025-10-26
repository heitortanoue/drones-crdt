package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPTCPSender implements TCPSender using an HTTP client
type HTTPTCPSender struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPTCPSender creates a new TCP sender via HTTP
func NewHTTPTCPSender(timeout time.Duration) *HTTPTCPSender {
	return &HTTPTCPSender{
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// SendDelta sends a delta to the given URL via POST /delta
func (hts *HTTPTCPSender) SendDelta(msgType string, url string, delta DeltaMsg) error {
	// Prepare JSON payload
	payload, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("failed to serialize delta: %v", err)
	}

	// Build full URL
	fullURL := fmt.Sprintf("%s/delta", url)

	// Create request
	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "drone-gossip/1.0")
	req.Header.Set("X-Message-Type", msgType)
	req.Header.Set("X-Drone-ID", delta.SenderID)
	req.Header.Set("X-Gossip-TTL", fmt.Sprintf("%d", delta.TTL))
	req.Header.Set("X-Message-ID", delta.ID.String())
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", delta.Timestamp))

	// Send request
	resp, err := hts.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP status %d when sending delta", resp.StatusCode)
	}

	return nil
}
