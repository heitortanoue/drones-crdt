package gossip

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

func TestHTTPTCPSender_NewHTTPTCPSender(t *testing.T) {
	timeout := 5 * time.Second
	sender := NewHTTPTCPSender(timeout)

	if sender == nil {
		t.Fatal("NewHTTPTCPSender não deveria retornar nil")
	}

	if sender.timeout != timeout {
		t.Errorf("Esperado timeout %v, obtido %v", timeout, sender.timeout)
	}

	if sender.client == nil {
		t.Error("Cliente HTTP não deveria ser nil")
	}

	if sender.client.Timeout != timeout {
		t.Errorf("Timeout do cliente HTTP deveria ser %v, obtido %v", timeout, sender.client.Timeout)
	}
}

func TestHTTPTCPSender_SendDelta_Success(t *testing.T) {
	// Cria servidor mock que aceita deltas
	var receivedDelta DeltaMsg
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Esperado método POST, obtido %s", r.Method)
		}

		if r.URL.Path != "/delta" {
			t.Errorf("Esperado path /delta, obtido %s", r.URL.Path)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Esperado Content-Type application/json, obtido %s", contentType)
		}

		userAgent := r.Header.Get("User-Agent")
		if userAgent != "drone-gossip/1.0" {
			t.Errorf("Esperado User-Agent drone-gossip/1.0, obtido %s", userAgent)
		}

		// Lê e decodifica o body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Erro ao ler body: %v", err)
			return
		}

		err = json.Unmarshal(body, &receivedDelta)
		if err != nil {
			t.Errorf("Erro ao decodificar JSON: %v", err)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "received"}`))
	}))
	defer server.Close()

	// Cria sender e delta de teste
	sender := NewHTTPTCPSender(5 * time.Second)
	deltaID := uuid.New()
	testDelta := DeltaMsg{
		ID:        deltaID,
		TTL:       3,
		SenderID:  "test-drone",
		Timestamp: time.Now().UnixMilli(),
		Data: sensor.SensorDelta{
			ID:        deltaID,
			SensorID:  "test-sensor",
			Value:     42.5,
			Timestamp: time.Now().UnixMilli(),
			DroneID:   "test-drone",
		},
	}

	// Envia delta
	err := sender.SendDelta(server.URL, testDelta)
	if err != nil {
		t.Errorf("SendDelta não deveria falhar: %v", err)
	}

	// Verifica se delta foi recebido corretamente
	if receivedDelta.ID != testDelta.ID {
		t.Errorf("ID do delta recebido incorreto. Esperado %s, obtido %s", testDelta.ID, receivedDelta.ID)
	}
	if receivedDelta.TTL != testDelta.TTL {
		t.Errorf("TTL do delta recebido incorreto. Esperado %d, obtido %d", testDelta.TTL, receivedDelta.TTL)
	}
	if receivedDelta.SenderID != testDelta.SenderID {
		t.Errorf("SenderID do delta recebido incorreto. Esperado %s, obtido %s", testDelta.SenderID, receivedDelta.SenderID)
	}
}

func TestHTTPTCPSender_SendDelta_ServerError(t *testing.T) {
	// Servidor que retorna erro
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	sender := NewHTTPTCPSender(5 * time.Second)
	testDelta := DeltaMsg{
		ID:       uuid.New(),
		TTL:      3,
		SenderID: "test-drone",
	}

	err := sender.SendDelta(server.URL, testDelta)
	if err == nil {
		t.Error("SendDelta deveria falhar com server error")
	}

	// Verifica se a mensagem de erro contém informação útil
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Erro deveria mencionar status 500: %v", err)
	}
}

func TestHTTPTCPSender_SendDelta_ConnectionRefused(t *testing.T) {
	sender := NewHTTPTCPSender(1 * time.Second)
	testDelta := DeltaMsg{
		ID:       uuid.New(),
		TTL:      3,
		SenderID: "test-drone",
	}

	// Tenta conectar em porta que não existe
	err := sender.SendDelta("http://localhost:99999", testDelta)
	if err == nil {
		t.Error("SendDelta deveria falhar com connection refused")
	}

	// Verifica se erro contém informação sobre conexão
	if !strings.Contains(err.Error(), "erro ao enviar request") {
		t.Errorf("Erro deveria mencionar falha na conexão: %v", err)
	}
}

func TestHTTPTCPSender_SendDelta_Timeout(t *testing.T) {
	// Servidor que demora para responder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Demora mais que o timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Sender com timeout muito baixo
	sender := NewHTTPTCPSender(100 * time.Millisecond)
	testDelta := DeltaMsg{
		ID:       uuid.New(),
		TTL:      3,
		SenderID: "test-drone",
	}

	err := sender.SendDelta(server.URL, testDelta)
	if err == nil {
		t.Error("SendDelta deveria falhar com timeout")
	}

	// Verifica se erro é relacionado a timeout
	if !strings.Contains(err.Error(), "erro ao enviar request") {
		t.Errorf("Erro deveria mencionar timeout: %v", err)
	}
}

func TestHTTPTCPSender_SendDelta_InvalidURL(t *testing.T) {
	sender := NewHTTPTCPSender(5 * time.Second)

	testDelta := DeltaMsg{
		ID:       uuid.New(),
		TTL:      3,
		SenderID: "test-drone",
	}

	// URL inválida que cause erro na criação do request
	err := sender.SendDelta(":", testDelta)
	if err == nil {
		t.Error("SendDelta deveria falhar com URL inválida")
	}

	// Verifica se erro menciona problema na criação do request
	if !strings.Contains(err.Error(), "erro ao criar request") {
		t.Errorf("Erro deveria mencionar problema na criação do request: %v", err)
	}
}

func TestHTTPTCPSender_SendDelta_MultipleRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"status": "received", "request_number": %d}`, requestCount)
		w.Write([]byte(response))
	}))
	defer server.Close()

	sender := NewHTTPTCPSender(5 * time.Second)

	// Envia múltiplos deltas
	for i := 0; i < 5; i++ {
		testDelta := DeltaMsg{
			ID:       uuid.New(),
			TTL:      3,
			SenderID: fmt.Sprintf("test-drone-%d", i),
		}

		err := sender.SendDelta(server.URL, testDelta)
		if err != nil {
			t.Errorf("SendDelta %d não deveria falhar: %v", i, err)
		}
	}

	if requestCount != 5 {
		t.Errorf("Esperado 5 requests, obtido %d", requestCount)
	}
}

func TestHTTPTCPSender_SendDelta_ConcurrentRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Pequeno delay para simular processamento
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "received"}`))
	}))
	defer server.Close()

	sender := NewHTTPTCPSender(5 * time.Second)
	numGoroutines := 10

	// Canal para coletar erros
	errCh := make(chan error, numGoroutines)

	// Envia deltas concorrentemente
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			testDelta := DeltaMsg{
				ID:       uuid.New(),
				TTL:      3,
				SenderID: fmt.Sprintf("test-drone-%d", routineID),
			}

			err := sender.SendDelta(server.URL, testDelta)
			errCh <- err
		}(i)
	}

	// Coleta resultados
	errorCount := 0
	for i := 0; i < numGoroutines; i++ {
		err := <-errCh
		if err != nil {
			errorCount++
			t.Errorf("Request concurrent %d falhou: %v", i, err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Tiveram %d erros em requests concorrentes", errorCount)
	}

	// Como há concorrência, o count pode variar, mas deve estar próximo
	if requestCount < numGoroutines-1 || requestCount > numGoroutines {
		t.Errorf("Request count inesperado: %d (esperado próximo de %d)", requestCount, numGoroutines)
	}
}
