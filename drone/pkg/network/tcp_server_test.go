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

// Helper function to find a free TCP port
func findFreeTCPPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// Helper function to make HTTP requests
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
		t.Fatal("NewTCPServer should not return nil")
	}

	if server.droneID != droneID {
		t.Errorf("Expected DroneID %s, got %s", droneID, server.droneID)
	}

	if server.port != port {
		t.Errorf("Expected port %d, got %d", port, server.port)
	}

	if server.mux == nil {
		t.Error("Mux should not be nil")
	}

	if server.server == nil {
		t.Error("HTTP server should not be nil")
	}

	expectedAddr := fmt.Sprintf(":%d", port)
	if server.server.Addr != expectedAddr {
		t.Errorf("Expected server address %s, got %s", expectedAddr, server.server.Addr)
	}
}

func TestTCPServer_HealthEndpoint(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Could not find a free TCP port")
	}

	droneID := "health-test-drone"
	server := NewTCPServer(droneID, port)

	// Start server in a goroutine
	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Error starting server: %v", err)
		}
	}()

	// Wait for server to initialize
	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Request /health
	url := fmt.Sprintf("http://localhost:%d/health", port)
	resp, err := makeHTTPRequest("GET", url)
	if err != nil {
		t.Fatalf("Error making request to /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		t.Fatalf("Error decoding JSON: %v", err)
	}

	expectedFields := []string{"drone_id", "status", "port"}
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Field %s should exist in response", field)
		}
	}

	if response["drone_id"] != droneID {
		t.Errorf("Expected drone_id %s, got %v", droneID, response["drone_id"])
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", response["status"])
	}

	if response["port"] != float64(port) { // JSON decodes numbers as float64
		t.Errorf("Expected port %d, got %v", port, response["port"])
	}
}

func TestTCPServer_NotImplementedEndpoints(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Could not find a free TCP port")
	}

	server := NewTCPServer("not-impl-test", port)

	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Error starting server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Endpoints that should return "not implemented"
	endpoints := []string{"/sensor", "/delta", "/state", "/stats", "/cleanup"}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:%d%s", port, endpoint)
			resp, err := makeHTTPRequest("GET", url)
			if err != nil {
				t.Fatalf("Error making request to %s: %v", endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusNotImplemented {
				t.Errorf("Expected status %d, got %d", http.StatusNotImplemented, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Error reading response body: %v", err)
			}

			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			if err != nil {
				t.Fatalf("Error decoding JSON: %v", err)
			}

			expectedFields := []string{"error", "feature", "phase"}
			for _, field := range expectedFields {
				if _, exists := response[field]; !exists {
					t.Errorf("Field %s should exist in response for %s", field, endpoint)
				}
			}

			if response["error"] != "Not implemented" {
				t.Errorf("Expected error 'Not implemented', got %v", response["error"])
			}
		})
	}
}

func TestTCPServer_CustomHandlers(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Could not find a free TCP port")
	}

	server := NewTCPServer("custom-handlers-test", port)

	// Define custom handlers
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
			t.Errorf("Error starting server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// Test each custom handler
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
				t.Fatalf("Error making request to %s: %v", tc.endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
			}

			if !*tc.called {
				t.Errorf("Custom handler for %s was not called", tc.endpoint)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Error reading body: %v", err)
			}

			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			if err != nil {
				t.Fatalf("Error decoding JSON: %v", err)
			}

			if response["message"] != tc.expectedMsg {
				t.Errorf("Expected message %s, got %v", tc.expectedMsg, response["message"])
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
			t.Errorf("Field %s should exist in GetStats", field)
		}
	}

	if stats["tcp_port"] != port {
		t.Errorf("Expected tcp_port %d, got %v", port, stats["tcp_port"])
	}

	if stats["drone_id"] != droneID {
		t.Errorf("Expected drone_id %s, got %v", droneID, stats["drone_id"])
	}
}

func TestTCPServer_ConcurrentRequests(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Could not find a free TCP port")
	}

	server := NewTCPServer("concurrent-test", port)

	// Handler simulating processing
	requestCount := 0
	var mutex sync.Mutex
	server.SensorHandler = func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requestCount++
		currentCount := requestCount
		mutex.Unlock()

		// Simulate processing delay
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
			t.Errorf("Error starting server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Make multiple concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			url := fmt.Sprintf("http://localhost:%d/sensor", port)
			resp, err := makeHTTPRequest("GET", url)
			if err != nil {
				t.Errorf("Error in concurrent request %d: %v", id, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Unexpected status code in request %d: %d", id, resp.StatusCode)
			}
		}(i)
	}

	wg.Wait()

	mutex.Lock()
	finalCount := requestCount
	mutex.Unlock()

	if finalCount != numGoroutines {
		t.Errorf("Expected %d requests processed, got %d", numGoroutines, finalCount)
	}
}

func TestTCPServer_ErrorHandling(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Could not find a free TCP port")
	}

	server1 := NewTCPServer("error-test-1", port)
	server2 := NewTCPServer("error-test-2", port)

	// Start first server
	go func() {
		err := server1.Start()
		if err != nil && err != http.ErrServerClosed {
			// Expected error when stopped
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Try to start second server on same port (should fail)
	err := server2.Start()
	if err == nil {
		t.Error("Second server should fail when using occupied port")
		server2.Stop()
	}

	// Stop first server
	err = server1.Stop()
	if err != nil {
		t.Errorf("Error stopping server: %v", err)
	}

	// Try request after server stopped
	time.Sleep(100 * time.Millisecond)
	url := fmt.Sprintf("http://localhost:%d/health", port)
	_, err = makeHTTPRequest("GET", url)
	if err == nil {
		t.Error("Request should fail for stopped server")
	}
}

func TestTCPServer_HTTPMethods(t *testing.T) {
	port := findFreeTCPPort()
	if port == 0 {
		t.Fatal("Could not find a free TCP port")
	}

	server := NewTCPServer("methods-test", port)

	// Handler that echoes method
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
			t.Errorf("Error starting server: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	methods := []string{"GET", "POST", "PUT", "DELETE"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:%d/sensor", port)
			resp, err := makeHTTPRequest(method, url)
			if err != nil {
				t.Fatalf("Error making %s request: %v", method, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Unexpected status code for %s: %d", method, resp.StatusCode)
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Error reading body: %v", err)
			}

			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			if err != nil {
				t.Fatalf("Error decoding JSON: %v", err)
			}

			if response["method"] != method {
				t.Errorf("Expected method %s, got %v", method, response["method"])
			}
		})
	}
}

func TestTCPServer_Stop_BeforeStart(t *testing.T) {
	server := NewTCPServer("stop-before-start", 8888)

	// Stop before ever starting
	err := server.Stop()
	if err != nil {
		t.Errorf("Stop should not return error for server not started: %v", err)
	}
}