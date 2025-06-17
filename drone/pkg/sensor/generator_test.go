package sensor

import (
	"testing"
	"time"
)

func TestSensorGenerator_BasicFunctionality(t *testing.T) {
	deltaSet := NewDeltaSet()
	generator := NewSensorGenerator("test-drone", deltaSet, 100*time.Millisecond)

	// Verifica estado inicial
	if generator.running {
		t.Error("Generator não deveria estar rodando inicialmente")
	}

	stats := generator.GetStats()
	if stats["drone_id"] != "test-drone" {
		t.Error("DroneID incorreto nas estatísticas")
	}

	// Inicia o gerador
	generator.Start()

	if !generator.running {
		t.Error("Generator deveria estar rodando após Start()")
	}

	// Aguarda algumas gerações
	time.Sleep(350 * time.Millisecond)

	// Para o gerador
	generator.Stop()

	if generator.running {
		t.Error("Generator não deveria estar rodando após Stop()")
	}

	// Verifica se deltas foram gerados
	if deltaSet.Size() < 2 {
		t.Errorf("Esperado pelo menos 2 deltas gerados, obtido %d", deltaSet.Size())
	}

	// Verifica se os deltas têm o drone ID correto
	all := deltaSet.GetAll()
	for _, delta := range all {
		if delta.DroneID != "test-drone" {
			t.Errorf("Delta com DroneID incorreto: %s", delta.DroneID)
		}
		if delta.Value < 0 || delta.Value > 100 {
			t.Errorf("Valor de umidade fora do range válido: %f", delta.Value)
		}
	}
}

func TestSensorGenerator_SensorAreas(t *testing.T) {
	deltaSet := NewDeltaSet()
	generator := NewSensorGenerator("test-drone", deltaSet, time.Second)

	areas := generator.GetSensorAreas()
	expectedAreas := []string{
		"area-test-drone-A",
		"area-test-drone-B",
		"area-test-drone-C",
	}

	if len(areas) != len(expectedAreas) {
		t.Errorf("Número de áreas incorreto: esperado %d, obtido %d", len(expectedAreas), len(areas))
	}

	// Verifica se as áreas são criadas corretamente
	for i, expected := range expectedAreas {
		if areas[i] != expected {
			t.Errorf("Área %d incorreta: esperado %s, obtido %s", i, expected, areas[i])
		}
	}

	// Testa adicionar nova área
	generator.AddSensorArea("custom-area")
	areasUpdated := generator.GetSensorAreas()

	if len(areasUpdated) != 4 {
		t.Errorf("Após adicionar área, esperado 4, obtido %d", len(areasUpdated))
	}

	if areasUpdated[3] != "custom-area" {
		t.Errorf("Nova área não foi adicionada corretamente: %s", areasUpdated[3])
	}
}

func TestSensorGenerator_IntervalUpdate(t *testing.T) {
	deltaSet := NewDeltaSet()
	generator := NewSensorGenerator("test-drone", deltaSet, time.Second)

	initialInterval := generator.interval
	if initialInterval != time.Second {
		t.Errorf("Intervalo inicial incorreto: %v", initialInterval)
	}

	// Atualiza intervalo
	newInterval := 500 * time.Millisecond
	generator.SetInterval(newInterval)

	if generator.interval != newInterval {
		t.Errorf("Intervalo não foi atualizado: esperado %v, obtido %v", newInterval, generator.interval)
	}

	stats := generator.GetStats()
	if stats["interval_sec"] != newInterval.Seconds() {
		t.Errorf("Estatísticas não refletem novo intervalo: %v", stats["interval_sec"])
	}
}

func TestSensorGenerator_ConcurrentStartStop(t *testing.T) {
	deltaSet := NewDeltaSet()
	generator := NewSensorGenerator("test-drone", deltaSet, 50*time.Millisecond)

	// Testa múltiplos starts
	generator.Start()
	generator.Start()
	generator.Start()

	if !generator.running {
		t.Error("Generator deveria estar rodando")
	}

	// Aguarda um pouco
	time.Sleep(150 * time.Millisecond)

	// Testa múltiplos stops
	generator.Stop()
	generator.Stop()
	generator.Stop()

	if generator.running {
		t.Error("Generator não deveria estar rodando")
	}

	// Verifica se deltas foram gerados
	if deltaSet.Size() < 1 {
		t.Errorf("Esperado pelo menos 1 delta, obtido %d", deltaSet.Size())
	}
}

func TestSensorAPI_Integration(t *testing.T) {
	api := NewSensorAPI("integration-test", 100*time.Millisecond)

	// Testa estado inicial
	state := api.GetState()
	if len(state) != 0 {
		t.Errorf("Estado inicial deveria estar vazio, obtido %d deltas", len(state))
	}

	// Inicia coleta automática
	api.Start()

	// Adiciona leitura manual
	reading := SensorReading{
		SensorID:  "manual-test",
		Timestamp: time.Now().UnixMilli(),
		Value:     42.5,
	}

	delta := api.AddManualReading(reading)
	if delta.SensorID != reading.SensorID {
		t.Error("Leitura manual não foi processada corretamente")
	}

	// Aguarda coleta automática
	time.Sleep(250 * time.Millisecond)

	// Verifica se dados foram coletados
	state = api.GetState()
	if len(state) < 2 {
		t.Errorf("Esperado pelo menos 2 deltas (1 manual + automáticos), obtido %d", len(state))
	}

	// Testa leituras mais recentes
	latest := api.GetLatestReadings()
	if len(latest) < 2 {
		t.Errorf("Esperado pelo menos 2 sensores únicos, obtido %d", len(latest))
	}

	// Verifica se leitura manual está presente
	if _, exists := latest["manual-test"]; !exists {
		t.Error("Leitura manual não encontrada nas leituras mais recentes")
	}

	// Para coleta
	api.Stop()

	// Testa estatísticas
	stats := api.GetStats()
	if stats["drone_id"] != "integration-test" {
		t.Error("DroneID incorreto nas estatísticas da API")
	}

	deltaStats := stats["delta_set"].(map[string]interface{})
	if deltaStats["total_deltas"].(int) < 2 {
		t.Error("Estatísticas não refletem deltas criados")
	}
}

func TestSensorAPI_MergeBatch(t *testing.T) {
	api := NewSensorAPI("merge-test", time.Hour) // Intervalo longo para não interferir

	// Cria batch de deltas remotos
	remoteDelta1 := NewSensorDelta("remote-drone", "remote-sensor-1", 75.0)
	remoteDelta2 := NewSensorDelta("remote-drone", "remote-sensor-2", 85.0)

	batch := DeltaBatch{
		SenderID: "remote-drone",
		Deltas:   []SensorDelta{remoteDelta1, remoteDelta2},
	}

	// Faz merge
	mergedCount := api.MergeBatch(batch)

	if mergedCount != 2 {
		t.Errorf("Esperado merge de 2 deltas, obtido %d", mergedCount)
	}

	// Verifica se deltas foram integrados
	state := api.GetState()
	if len(state) != 2 {
		t.Errorf("Estado deveria ter 2 deltas, obtido %d", len(state))
	}

	// Testa merge do mesmo batch (não deve adicionar duplicatas)
	mergedCount2 := api.MergeBatch(batch)
	if mergedCount2 != 0 {
		t.Errorf("Segundo merge deveria retornar 0, obtido %d", mergedCount2)
	}

	state2 := api.GetState()
	if len(state2) != 2 {
		t.Errorf("Estado após segundo merge deveria ter 2 deltas, obtido %d", len(state2))
	}
}

func TestSensorAPI_CleanupOldData(t *testing.T) {
	api := NewSensorAPI("cleanup-test", time.Hour)

	// Adiciona algumas leituras com timestamps específicos
	now := time.Now().UnixMilli()
	oldTime := now - 10000 // 10 segundos atrás

	for i := 0; i < 5; i++ {
		reading := SensorReading{
			SensorID:  "test-sensor",
			Timestamp: oldTime + int64(i*1000), // Espaça 1 segundo entre cada
			Value:     float64(i * 10),
		}
		api.AddManualReading(reading)
	}

	// Verifica estado inicial
	if len(api.GetState()) != 5 {
		t.Errorf("Esperado 5 deltas iniciais, obtido %d", len(api.GetState()))
	}

	// Cleanup removendo dados mais antigos que 5 segundos
	// Isso deve remover deltas com timestamp < (now - 5000)
	removedCount := api.CleanupOldData(5 * time.Second)

	// Como os deltas foram criados 10-6 segundos atrás, todos deveriam ser removidos
	if removedCount < 4 {
		t.Errorf("Esperado remoção de pelo menos 4 deltas, obtido %d", removedCount)
	}

	if len(api.GetState()) > 1 {
		t.Errorf("Estado após cleanup deveria ter no máximo 1 delta, obtido %d deltas", len(api.GetState()))
	}
}
