package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

func TestIntegration_Fase2_CompleteWorkflow(t *testing.T) {
	// Este teste verifica todo o fluxo da Fase 2 de forma integrada

	// 1. Configura ambiente de teste
	testDroneID := "test-integration-drone"
	testSampleInterval := 200 * time.Millisecond

	// 2. Cria sistema de sensores
	sensorAPI := sensor.NewSensorAPI(testDroneID, testSampleInterval)

	// 3. Inicia coleta automática
	sensorAPI.Start()
	defer sensorAPI.Stop()

	// 4. Aguarda algumas coletas automáticas
	time.Sleep(500 * time.Millisecond)

	// 5. Verifica se coleta automática está funcionando
	state := sensorAPI.GetState()
	if len(state) < 2 {
		t.Errorf("Coleta automática falhou: esperado >= 2 deltas, obtido %d", len(state))
	}

	// 6. Testa adição manual de leitura
	manualReading := sensor.SensorReading{
		SensorID:  "manual-integration-sensor",
		Timestamp: sensor.GenerateTimestamp(),
		Value:     99.9,
	}

	manualDelta := sensorAPI.AddManualReading(manualReading)
	if manualDelta.Value != 99.9 {
		t.Errorf("Leitura manual incorreta: esperado 99.9, obtido %f", manualDelta.Value)
	}

	// 7. Verifica se leitura manual foi integrada
	updatedState := sensorAPI.GetState()
	if len(updatedState) <= len(state) {
		t.Error("Leitura manual não foi integrada ao estado")
	}

	// 8. Testa merge de dados de drone remoto
	remoteBatch := sensor.DeltaBatch{
		SenderID: "remote-test-drone",
		Deltas: []sensor.SensorDelta{
			sensor.NewSensorDelta("remote-test-drone", "remote-sensor-1", 88.8),
			sensor.NewSensorDelta("remote-test-drone", "remote-sensor-2", 77.7),
		},
	}

	mergedCount := sensorAPI.MergeBatch(remoteBatch)
	if mergedCount != 2 {
		t.Errorf("Merge falhou: esperado 2 deltas, obtido %d", mergedCount)
	}

	// 9. Verifica estado final
	finalState := sensorAPI.GetState()
	latest := sensorAPI.GetLatestReadings()

	expectedMinDeltas := len(state) + 1 + 2 // automáticos + manual + remotos
	if len(finalState) < expectedMinDeltas {
		t.Errorf("Estado final incorreto: esperado >= %d deltas, obtido %d",
			expectedMinDeltas, len(finalState))
	}

	// 10. Verifica se todos os tipos de sensores estão presentes
	sensorsFound := make(map[string]bool)
	for sensorID := range latest {
		if strings.Contains(sensorID, testDroneID) {
			sensorsFound["automatic"] = true
		}
		if sensorID == "manual-integration-sensor" {
			sensorsFound["manual"] = true
		}
		if strings.Contains(sensorID, "remote-sensor") {
			sensorsFound["remote"] = true
		}
	}

	if !sensorsFound["automatic"] {
		t.Error("Sensores automáticos não encontrados")
	}
	if !sensorsFound["manual"] {
		t.Error("Sensor manual não encontrado")
	}
	if !sensorsFound["remote"] {
		t.Error("Sensores remotos não encontrados")
	}

	// 11. Testa estatísticas
	stats := sensorAPI.GetStats()

	deltaStats := stats["delta_set"].(map[string]interface{})
	if deltaStats["total_deltas"].(int) < expectedMinDeltas {
		t.Errorf("Estatísticas incorretas: total_deltas %v", deltaStats["total_deltas"])
	}

	generatorStats := stats["generator"].(map[string]interface{})
	if !generatorStats["running"].(bool) {
		t.Error("Generator deveria estar rodando")
	}

	// 12. Testa cleanup
	removedCount := sensorAPI.CleanupOldData(time.Hour) // Remove dados > 1 hora
	if removedCount > 0 {
		t.Errorf("Cleanup removeu dados recentes: %d deltas removidos", removedCount)
	}

	// 13. Testa IDs e funcionalidades CRDT
	allIDs := sensorAPI.GetAllDeltaIDs()
	if len(allIDs) != len(finalState) {
		t.Errorf("GetAllDeltaIDs inconsistente: IDs=%d, deltas=%d",
			len(allIDs), len(finalState))
	}

	// Verifica funcionalidade GetMissingDeltas
	someIDs := allIDs[:min(3, len(allIDs))]
	missing := sensorAPI.GetMissingDeltas(someIDs)
	if len(missing) > 0 {
		t.Errorf("Não deveria haver IDs missing para IDs conhecidos: %d missing",
			len(missing))
	}

	// Testa com IDs inexistentes
	fakeIDs := []uuid.UUID{uuid.New(), uuid.New()}
	missing = sensorAPI.GetMissingDeltas(fakeIDs)
	if len(missing) != 2 {
		t.Errorf("Deveria reportar 2 IDs missing, obtido %d", len(missing))
	}

	fmt.Printf("✅ Teste de Integração Fase 2 completo:\n")
	fmt.Printf("   - Coleta automática: %d deltas gerados\n", len(state))
	fmt.Printf("   - Leitura manual: integrada com sucesso\n")
	fmt.Printf("   - Merge remoto: %d deltas integrados\n", mergedCount)
	fmt.Printf("   - Estado final: %d deltas total\n", len(finalState))
	fmt.Printf("   - Sensores únicos: %d\n", len(latest))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Simulação de teste HTTP (para documentação)
func TestIntegration_HTTPEndpoints_Documentation(t *testing.T) {
	// Este teste documenta como os endpoints HTTP deveriam funcionar
	// Em um teste real, precisaríamos de um servidor HTTP rodando

	testCases := []struct {
		name           string
		method         string
		endpoint       string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "Health Check",
			method:         "GET",
			endpoint:       "/health",
			expectedStatus: 200,
			description:    "Verifica se drone está funcionando",
		},
		{
			name:           "Add Manual Reading",
			method:         "POST",
			endpoint:       "/sensor",
			payload:        `{"sensor_id": "test-sensor", "value": 42.5}`,
			expectedStatus: 200,
			description:    "Adiciona leitura manual de sensor",
		},
		{
			name:           "Merge Remote Deltas",
			method:         "POST",
			endpoint:       "/delta",
			payload:        `{"sender_id": "remote-drone", "deltas": [...]}`,
			expectedStatus: 200,
			description:    "Recebe e integra deltas de outro drone",
		},
		{
			name:           "Get Current State",
			method:         "GET",
			endpoint:       "/state",
			expectedStatus: 200,
			description:    "Retorna estado atual do CRDT",
		},
		{
			name:           "Get Statistics",
			method:         "GET",
			endpoint:       "/stats",
			expectedStatus: 200,
			description:    "Retorna estatísticas do sistema",
		},
		{
			name:           "Cleanup Old Data",
			method:         "POST",
			endpoint:       "/cleanup",
			expectedStatus: 200,
			description:    "Remove dados antigos",
		},
	}

	fmt.Printf("📋 Endpoints HTTP implementados na Fase 2:\n")
	for _, tc := range testCases {
		fmt.Printf("   %s %s - %s\n", tc.method, tc.endpoint, tc.description)
		if tc.payload != "" {
			fmt.Printf("     Payload exemplo: %s\n", tc.payload)
		}
	}
}
