package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/heitortanoue/tcc/sensor"
)

func TestNewDroneServer(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	if server.crdt == nil {
		t.Error("CRDT não deve ser nil")
	}

	if server.port != 8080 {
		t.Errorf("Esperado porta 8080, obtido %d", server.port)
	}
}

func TestHandleSensor(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	// Prepara request
	reading := sensor.SensorReading{
		SensorID:  "talhao-test",
		Timestamp: time.Now().UnixMilli(),
		Value:     22.5,
	}

	jsonData, _ := json.Marshal(reading)
	req := httptest.NewRequest(http.MethodPost, "/sensor", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Executa request
	recorder := httptest.NewRecorder()
	server.handleSensor(recorder, req)

	// Verifica resposta
	if recorder.Code != http.StatusCreated {
		t.Errorf("Esperado status 201, obtido %d", recorder.Code)
	}

	var response SensorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Errorf("Erro ao decodificar resposta: %v", err)
	}

	if response.Delta.SensorID != "talhao-test" {
		t.Errorf("Esperado sensor ID 'talhao-test', obtido %s", response.Delta.SensorID)
	}

	if response.Delta.Value != 22.5 {
		t.Errorf("Esperado valor 22.5, obtido %f", response.Delta.Value)
	}
}

func TestHandleGetDeltas(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	// Adiciona alguns deltas
	reading1 := sensor.SensorReading{SensorID: "test1", Timestamp: time.Now().UnixMilli(), Value: 20.0}
	reading2 := sensor.SensorReading{SensorID: "test2", Timestamp: time.Now().UnixMilli(), Value: 25.0}

	server.crdt.AddDelta(reading1)
	server.crdt.AddDelta(reading2)

	// Executa request
	req := httptest.NewRequest(http.MethodGet, "/deltas", nil)
	recorder := httptest.NewRecorder()
	server.handleGetDeltas(recorder, req)

	// Verifica resposta
	if recorder.Code != http.StatusOK {
		t.Errorf("Esperado status 200, obtido %d", recorder.Code)
	}

	var response DeltasResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Errorf("Erro ao decodificar resposta: %v", err)
	}

	if len(response.Pending) != 2 {
		t.Errorf("Esperado 2 deltas pendentes, obtido %d", len(response.Pending))
	}
}

func TestHandlePostDelta(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	// Prepara batch de deltas
	batch := sensor.DeltaBatch{
		SenderID: "remote-drone",
		Deltas: []sensor.SensorDelta{
			{
				DroneID:   "remote-drone",
				SensorID:  "remote-sensor",
				Timestamp: time.Now().UnixMilli(),
				Value:     30.0,
			},
		},
	}

	jsonData, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPost, "/delta", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Executa request
	recorder := httptest.NewRecorder()
	server.handlePostDelta(recorder, req)

	// Verifica resposta
	if recorder.Code != http.StatusOK {
		t.Errorf("Esperado status 200, obtido %d", recorder.Code)
	}

	var response MergeResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Errorf("Erro ao decodificar resposta: %v", err)
	}

	if response.MergedCount != 1 {
		t.Errorf("Esperado 1 delta merged, obtido %d", response.MergedCount)
	}

	if response.CurrentTotal != 1 {
		t.Errorf("Esperado total 1, obtido %d", response.CurrentTotal)
	}
}

func TestHandleGetState(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	// Adiciona alguns deltas
	reading := sensor.SensorReading{
		SensorID:  "state-test",
		Timestamp: time.Now().UnixMilli(),
		Value:     15.5,
	}
	server.crdt.AddDelta(reading)

	// Executa request
	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	recorder := httptest.NewRecorder()
	server.handleGetState(recorder, req)

	// Verifica resposta
	if recorder.Code != http.StatusOK {
		t.Errorf("Esperado status 200, obtido %d", recorder.Code)
	}

	var response StateResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Errorf("Erro ao decodificar resposta: %v", err)
	}

	if len(response.State) != 1 {
		t.Errorf("Esperado 1 item no state, obtido %d", len(response.State))
	}

	if response.State[0].SensorID != "state-test" {
		t.Errorf("Esperado sensor ID 'state-test', obtido %s", response.State[0].SensorID)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	// Testa método não permitido no endpoint /sensor
	req := httptest.NewRequest(http.MethodGet, "/sensor", nil)
	recorder := httptest.NewRecorder()
	server.handleSensor(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("Esperado status 405, obtido %d", recorder.Code)
	}
}

func TestInvalidJSON(t *testing.T) {
	server := NewDroneServer("test-drone", 8080)

	// Testa JSON inválido
	req := httptest.NewRequest(http.MethodPost, "/sensor", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	server.handleSensor(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Esperado status 400, obtido %d", recorder.Code)
	}
}
