package sensor

import (
	"testing"
	"time"
)

func TestFireSensorGenerator_BasicFunctionality(t *testing.T) {
	sensor := NewFireSensor("test-sensor", 100*time.Millisecond)

	// Verifica estado inicial
	if sensor.generator.running {
		t.Error("Generator não deveria estar rodando inicialmente")
	}

	stats := sensor.GetStats()
	if stats["sensor_id"] != "test-sensor" {
		t.Error("SensorID incorreto nas estatísticas")
	}

	// Inicia o gerador
	sensor.Start()

	if !sensor.generator.running {
		t.Error("Generator deveria estar rodando após Start()")
	}

	// Aguarda algumas gerações
	time.Sleep(350 * time.Millisecond)

	// Para o gerador
	sensor.Stop()

	if sensor.generator.running {
		t.Error("Generator não deveria estar rodando após Stop()")
	}

	// Verifica se leituras foram geradas
	readings := sensor.GetReadings()
	if len(readings) < 2 {
		t.Errorf("Esperado pelo menos 2 leituras geradas, obtido %d", len(readings))
	}

	// Verifica se as leituras estão na área correta
	for _, reading := range readings {
		if reading.X < sensor.generator.baseX || reading.X >= sensor.generator.baseX+sensor.generator.gridSize {
			t.Errorf("Coordenada X fora da área esperada: %d", reading.X)
		}
		if reading.Y < sensor.generator.baseY || reading.Y >= sensor.generator.baseY+sensor.generator.gridSize {
			t.Errorf("Coordenada Y fora da área esperada: %d", reading.Y)
		}
		if reading.Confidence < 0 || reading.Confidence > 100 {
			t.Errorf("Confiança fora do range válido: %f", reading.Confidence)
		}
		if reading.SensorID != "test-sensor" {
			t.Errorf("SensorID incorreto na leitura: %s", reading.SensorID)
		}
	}
}

func TestFireSensor_ManualReadings(t *testing.T) {
	sensor := NewFireSensor("manual-test-sensor", time.Hour) // Intervalo longo para não interferir

	// Adiciona leitura manual
	sensor.AddManualReading(15, 25, 85.5)

	readings := sensor.GetReadings()
	if len(readings) != 1 {
		t.Errorf("Esperado 1 leitura manual, obtido %d", len(readings))
	}

	reading := readings[0]
	if reading.X != 15 || reading.Y != 25 {
		t.Error("Coordenadas da leitura manual incorretas")
	}
	if reading.Confidence != 85.5 {
		t.Error("Confiança da leitura manual incorreta")
	}
	if reading.SensorID != "manual-test-sensor" {
		t.Error("SensorID da leitura manual incorreto")
	}
}

func TestFireSensor_GetAndClearReadings(t *testing.T) {
	sensor := NewFireSensor("clear-test-sensor", time.Hour)

	// Adiciona algumas leituras
	sensor.AddManualReading(10, 20, 75.0)
	sensor.AddManualReading(15, 25, 80.0)
	sensor.AddManualReading(20, 30, 90.0)

	// Verifica que temos 3 leituras
	readings := sensor.GetReadings()
	if len(readings) != 3 {
		t.Errorf("Esperado 3 leituras, obtido %d", len(readings))
	}

	// Obtém e limpa as leituras (simula envio para drone)
	clearedReadings := sensor.GetAndClearReadings()
	if len(clearedReadings) != 3 {
		t.Errorf("Esperado 3 leituras no retorno, obtido %d", len(clearedReadings))
	}

	// Verifica que a lista foi limpa
	remainingReadings := sensor.GetReadings()
	if len(remainingReadings) != 0 {
		t.Errorf("Esperado 0 leituras após limpeza, obtido %d", len(remainingReadings))
	}

	// Adiciona nova leitura após limpeza
	sensor.AddManualReading(5, 10, 60.0)
	newReadings := sensor.GetReadings()
	if len(newReadings) != 1 {
		t.Errorf("Esperado 1 nova leitura, obtido %d", len(newReadings))
	}
}
