package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Função auxiliar para encontrar uma porta TCP livre
func findFreeTCPPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// Função auxiliar para fazer requests HTTP
func makeHTTPRequest(method, url string) (*http.Response, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func TestTCPServer_NewTCPServer(t *testing.T) {
	droneID := "test-tcp-drone"
	port := 8080

	server := NewTCPServer(droneID, port)

	if server == nil {
		t.Fatal("NewTCPServer não deveria retornar nil")
	}

	if server.droneID != droneID {
		t.Errorf("DroneID esperado %s, obtido %s", droneID, server.droneID)
	}

	if server.port != port {
		t.Errorf("Porta esperada %d, obtida %d", port, server.port)
	}

	if server.mux == nil {
		t.Error("Mux não deveria ser nil")
	}

	if server.server == nil {
		t.Error("HTTP server não deveria ser nil")
	}

	expectedAddr := fmt.Sprintf(":%d", port)
	if server.server.Addr != expectedAddr {
		t.Errorf("Endereço do servidor esperado %s, obtido %s", expectedAddr, server.server.Addr)
	}
}

func TestTCPServer_HealthEndpoint(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta TCP livre")
	}

	droneID := "health-test-drone"
	server := NewTCPServer(droneID, port)

	// Inicia servidor em goroutine
	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Erro ao iniciar servidor: %v", err)
		}
	}()

	// Aguarda servidor inicializar
	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Faz request para /health
	url := fmt.Sprintf("http://localhost:%d/health", port)
	resp, err := makeHTTPRequest("GET", url)
	if err != nil {
		t.Fatalf("Erro ao fazer request para /health: %v", err)
	}
	defer resp.Body.Close()

	// Verifica status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code esperado %d, obtido %d", http.StatusOK, resp.StatusCode)
	}

	// Verifica content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type esperado application/json, obtido %s", contentType)
	}

	// Verifica conteúdo da resposta
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Erro ao ler body da resposta: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}

	expectedFields := []string{"drone_id", "status", "port"}
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Campo %s deveria existir na resposta", field)
		}
	}

	if response["drone_id"] != droneID {
		t.Errorf("drone_id esperado %s, obtido %v", droneID, response["drone_id"])
	}

	if response["status"] != "healthy" {
		t.Errorf("status esperado healthy, obtido %v", response["status"])
	}

	if response["port"] != float64(port) { // JSON decode números como float64
		t.Errorf("port esperado %d, obtido %v", port, response["port"])
	}
}

func TestTCPServer_NotImplementedEndpoints(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta TCP livre")
	}

	server := NewTCPServer("not-impl-test", port)

	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Erro ao iniciar servidor: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Lista de endpoints que devem retornar "not implemented"
	endpoints := []string{"/sensor", "/delta", "/state", "/stats", "/cleanup"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:%d%s", port, endpoint)
			resp, err := makeHTTPRequest("GET", url)
			if err != nil {
				t.Fatalf("Erro ao fazer request para %s: %v", endpoint, err)
			}
			defer resp.Body.Close()

			// Verifica status code
			if resp.StatusCode != http.StatusNotImplemented {
				t.Errorf("Status code esperado %d, obtido %d", http.StatusNotImplemented, resp.StatusCode)
			}

			// Verifica content type
			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type esperado application/json, obtido %s", contentType)
			}

			// Verifica conteúdo da resposta
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Erro ao ler body da resposta: %v", err)
			}

			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			if err != nil {
				t.Fatalf("Erro ao decodificar JSON: %v", err)
			}

			expectedFields := []string{"error", "feature", "phase"}
			for _, field := range expectedFields {
				if _, exists := response[field]; !exists {
					t.Errorf("Campo %s deveria existir na resposta de %s", field, endpoint)
				}
			}

			if response["error"] != "Not implemented" {
				t.Errorf("error esperado 'Not implemented', obtido %v", response["error"])
			}
		})
	}
}

func TestTCPServer_CustomHandlers(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta TCP livre")
	}

	server := NewTCPServer("custom-handlers-test", port)

	// Define handlers customizados
	sensorCalled := false
	server.SensorHandler = func(w http.ResponseWriter, r *http.Request) {
		sensorCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "custom sensor handler"})
	}

	deltaCalled := false
	server.DeltaHandler = func(w http.ResponseWriter, r *http.Request) {
		deltaCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "custom delta handler"})
	}

	stateCalled := false
	server.StateHandler = func(w http.ResponseWriter, r *http.Request) {
		stateCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "custom state handler"})
	}

	statsCalled := false
	server.StatsHandler = func(w http.ResponseWriter, r *http.Request) {
		statsCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "custom stats handler"})
	}

	cleanupCalled := false
	server.CleanupHandler = func(w http.ResponseWriter, r *http.Request) {
		cleanupCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "custom cleanup handler"})
	}

	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Erro ao iniciar servidor: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Testa cada handler customizado
	testCases := []struct {
		endpoint    string
		called      *bool
		expectedMsg string
	}{
		{"/sensor", &sensorCalled, "custom sensor handler"},
		{"/delta", &deltaCalled, "custom delta handler"},
		{"/state", &stateCalled, "custom state handler"},
		{"/stats", &statsCalled, "custom stats handler"},
		{"/cleanup", &cleanupCalled, "custom cleanup handler"},
	}

	for _, tc := range testCases {
		t.Run(tc.endpoint, func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:%d%s", port, tc.endpoint)
			resp, err := makeHTTPRequest("GET", url)
			if err != nil {
				t.Fatalf("Erro ao fazer request para %s: %v", tc.endpoint, err)
			}
			defer resp.Body.Close()

			// Verifica status code
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code esperado %d, obtido %d", http.StatusOK, resp.StatusCode)
			}

			// Verifica se handler foi chamado
			if !*tc.called {
				t.Errorf("Handler customizado para %s não foi chamado", tc.endpoint)
			}

			// Verifica conteúdo da resposta
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Erro ao ler body da resposta: %v", err)
			}

			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			if err != nil {
				t.Fatalf("Erro ao decodificar JSON: %v", err)
			}

			if response["message"] != tc.expectedMsg {
				t.Errorf("Mensagem esperada %s, obtida %v", tc.expectedMsg, response["message"])
			}
		})
	}
}

func TestTCPServer_GetStats(t *testing.T) {
	droneID := "stats-test"
	port := 9999
	server := NewTCPServer(droneID, port)

	stats := server.GetStats()

	expectedFields := []string{"tcp_port", "drone_id"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Campo %s deveria existir em GetStats", field)
		}
	}

	if stats["tcp_port"] != port {
		t.Errorf("tcp_port esperado %d, obtido %v", port, stats["tcp_port"])
	}

	if stats["drone_id"] != droneID {
		t.Errorf("drone_id esperado %s, obtido %v", droneID, stats["drone_id"])
	}
}

func TestTCPServer_ConcurrentRequests(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta TCP livre")
	}

	server := NewTCPServer("concurrent-test", port)

	// Handler que simula processamento
	requestCount := 0
	var mutex sync.Mutex
	server.SensorHandler = func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requestCount++
		currentCount := requestCount
		mutex.Unlock()

		// Simula processamento
		time.Sleep(10 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"request_id": currentCount,
			"message":    "processed",
		})
	}

	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Erro ao iniciar servidor: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Faz múltiplas requests concorrentes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			url := fmt.Sprintf("http://localhost:%d/sensor", port)
			resp, err := makeHTTPRequest("GET", url)
			if err != nil {
				t.Errorf("Erro na request concorrente %d: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code inesperado na request %d: %d", id, resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()

	// Verifica se todas as requests foram processadas
	mutex.Lock()
	finalCount := requestCount
	mutex.Unlock()

	if finalCount != numGoroutines {
		t.Errorf("Esperado %d requests processadas, obtido %d", numGoroutines, finalCount)
	}
}

func TestTCPServer_ErrorHandling(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta TCP livre")
	}

	server1 := NewTCPServer("error-test-1", port)
	server2 := NewTCPServer("error-test-2", port)

	// Inicia primeiro servidor
	go func() {
		err := server1.Start()
		if err != nil && err != http.ErrServerClosed {
			// Erro esperado quando servidor é parado
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Tenta iniciar segundo servidor na mesma porta (deveria falhar)
	err := server2.Start()
	if err == nil {
		t.Error("Segundo servidor deveria falhar ao usar porta ocupada")
		server2.Stop()
	}

	// Para primeiro servidor
	err = server1.Stop()
	if err != nil {
		t.Errorf("Erro ao parar servidor: %v", err)
	}

	// Tenta fazer request após servidor parado
	time.Sleep(100 * time.Millisecond)
	url := fmt.Sprintf("http://localhost:%d/health", port)
	_, err = makeHTTPRequest("GET", url)
	if err == nil {
		t.Error("Request deveria falhar para servidor parado")
	}
}

func TestTCPServer_HTTPMethods(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta TCP livre")
	}

	server := NewTCPServer("methods-test", port)

	// Handler que verifica método HTTP
	server.SensorHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		})
	}

	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Erro ao iniciar servidor: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Testa diferentes métodos HTTP
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:%d/sensor", port)
			resp, err := makeHTTPRequest(method, url)
			if err != nil {
				t.Fatalf("Erro ao fazer request %s: %v", method, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code inesperado para %s: %d", method, resp.StatusCode)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Erro ao ler body: %v", err)
			}

			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			if err != nil {
				t.Fatalf("Erro ao decodificar JSON: %v", err)
			}

			if response["method"] != method {
				t.Errorf("Método esperado %s, obtido %v", method, response["method"])
			}
		})
	}
}

func TestTCPServer_Stop_BeforeStart(t *testing.T) {
	server := NewTCPServer("stop-before-start", 8888)

	// Tenta parar servidor que nunca foi iniciado
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop não deveria retornar erro para servidor não iniciado: %v", err)
	}
}
