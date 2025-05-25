package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/heitortanoue/tcc/sensor"
)

/* ---------- fixture global ---------- */

var (
	testServer *DroneServer
)

var droneConfigMock = DroneConfig{
	DroneID: "test-drone",
	APIPort: 8080,
}

// TestMain é executado uma vez por arquivo _test.go
func TestMain(m *testing.M) {
	var err error
	testServer, err = NewDroneServer(droneConfigMock)
	if err != nil {
		fmt.Fprintf(os.Stderr, "falha ao criar DroneServer: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	testServer.Shutdown()
	os.Exit(code)
}

/* ---------- testes ---------- */

// resetCRDT cleans the shared CRDT before each test and registers a cleanup.
func resetCRDT(t *testing.T) {
	testServer.crdt.Reset()
	t.Cleanup(testServer.crdt.Reset)
}

func TestHandleSensor(t *testing.T) {
	resetCRDT(t)
	reading := sensor.SensorReading{
		SensorID:  "talhao-test",
		Timestamp: time.Now().UnixMilli(),
		Value:     22.5,
	}
	body, _ := json.Marshal(reading)

	req := httptest.NewRequest(http.MethodPost, "/sensor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testServer.handleSensor(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d", rec.Code)
	}

	var resp SensorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Delta.SensorID != "talhao-test" || resp.Delta.Value != 22.5 {
		t.Errorf("dados inesperados %+v", resp.Delta)
	}
}

func TestHandleGetDeltas(t *testing.T) {
	resetCRDT(t)
	// insere 2 leituras
	for i := 1; i <= 2; i++ {
		testServer.crdt.AddDelta(sensor.SensorReading{
			SensorID:  fmt.Sprintf("test%d", i),
			Timestamp: time.Now().UnixMilli(),
			Value:     20 + float64(i),
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/deltas", nil)
	rec := httptest.NewRecorder()

	testServer.handleGetDeltas(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}

	var resp DeltasResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Pending) != 2 {
		t.Errorf("esperado 2 deltas, obtido %d", len(resp.Pending))
	}
}

func TestHandlePostDelta(t *testing.T) {
	resetCRDT(t)
	batch := sensor.DeltaBatch{
		SenderID: "remote",
		Deltas: []sensor.SensorDelta{{
			DroneID:   "remote",
			SensorID:  "r-sensor",
			Timestamp: time.Now().UnixMilli(),
			Value:     30.0,
		}},
	}
	body, _ := json.Marshal(batch)

	req := httptest.NewRequest(http.MethodPost, "/delta", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testServer.handlePostDelta(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}

	var resp MergeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.MergedCount != 1 || resp.CurrentTotal < 1 {
		t.Errorf("merge incorreto: %+v", resp)
	}
}

func TestHandleGetState(t *testing.T) {
	resetCRDT(t)
	// garante pelo menos 1 leitura
	testServer.crdt.AddDelta(sensor.SensorReading{
		SensorID:  "state-test",
		Timestamp: time.Now().UnixMilli(),
		Value:     15.5,
	})

	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	rec := httptest.NewRecorder()

	testServer.handleGetState(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}

	var resp StateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.State) == 0 || resp.State[0].SensorID != "state-test" {
		t.Errorf("state inesperado %+v", resp.State)
	}
}

func TestInvalidJSON(t *testing.T) {
	resetCRDT(t)
	req := httptest.NewRequest(http.MethodPost, "/sensor", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	testServer.handleSensor(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400, obtido %d", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	resetCRDT(t)
	req := httptest.NewRequest(http.MethodGet, "/sensor", nil)
	rec := httptest.NewRecorder()

	testServer.handleSensor(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("esperado 405, obtido %d", rec.Code)
	}
}
