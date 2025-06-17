package gossip

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

// Mock implementations para testes

type MockNeighborGetter struct {
	neighbors []string
	mutex     sync.RWMutex
}

func NewMockNeighborGetter(neighbors []string) *MockNeighborGetter {
	return &MockNeighborGetter{neighbors: neighbors}
}

func (m *MockNeighborGetter) GetNeighborURLs() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]string, len(m.neighbors))
	copy(result, m.neighbors)
	return result
}

func (m *MockNeighborGetter) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.neighbors)
}

func (m *MockNeighborGetter) SetNeighbors(neighbors []string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.neighbors = neighbors
}

type MockTCPSender struct {
	sentMessages []DeltaMsg
	shouldFail   bool
	failURL      string
	mutex        sync.RWMutex
}

func NewMockTCPSender() *MockTCPSender {
	return &MockTCPSender{
		sentMessages: make([]DeltaMsg, 0),
	}
}

func (m *MockTCPSender) SendDelta(url string, delta DeltaMsg) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldFail && (m.failURL == "" || m.failURL == url) {
		return &MockSendError{URL: url}
	}

	m.sentMessages = append(m.sentMessages, delta)
	return nil
}

func (m *MockTCPSender) GetSentMessages() []DeltaMsg {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]DeltaMsg, len(m.sentMessages))
	copy(result, m.sentMessages)
	return result
}

func (m *MockTCPSender) SetShouldFail(fail bool, url string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldFail = fail
	m.failURL = url
}

func (m *MockTCPSender) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.sentMessages = make([]DeltaMsg, 0)
	m.shouldFail = false
	m.failURL = ""
}

type MockSendError struct {
	URL string
}

func (e *MockSendError) Error() string {
	return "mock send error to " + e.URL
}

// Helper functions

func createTestDelta(droneID string) sensor.SensorDelta {
	return sensor.SensorDelta{
		ID:        uuid.New(),
		SensorID:  "test-sensor",
		Value:     42.5,
		Timestamp: time.Now().UnixMilli(),
		DroneID:   droneID,
	}
}

// Testes

func TestDisseminationSystem_NewDisseminationSystem(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{"http://node1:8080", "http://node2:8080"})
	tcpSender := NewMockTCPSender()

	ds := NewDisseminationSystem("test-drone", 3, 4, neighborGetter, tcpSender)

	if ds == nil {
		t.Fatal("NewDisseminationSystem não deveria retornar nil")
	}

	if ds.droneID != "test-drone" {
		t.Errorf("Esperado droneID test-drone, obtido %s", ds.droneID)
	}

	if ds.fanout != 3 {
		t.Errorf("Esperado fanout 3, obtido %d", ds.fanout)
	}

	if ds.defaultTTL != 4 {
		t.Errorf("Esperado defaultTTL 4, obtido %d", ds.defaultTTL)
	}

	if ds.running {
		t.Error("Sistema não deveria estar running na criação")
	}

	if ds.cache == nil {
		t.Error("Cache não deveria ser nil")
	}

	if ds.cache.Size() != 0 {
		t.Errorf("Cache deveria estar vazio, size: %d", ds.cache.Size())
	}
}

func TestDisseminationSystem_StartStop(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{})
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 3, 4, neighborGetter, tcpSender)

	// Sistema deve começar parado
	if ds.IsRunning() {
		t.Error("Sistema deveria começar parado")
	}

	// Inicia sistema
	ds.Start()
	if !ds.IsRunning() {
		t.Error("Sistema deveria estar rodando após Start")
	}

	// Start múltiplos não devem causar problema
	ds.Start()
	if !ds.IsRunning() {
		t.Error("Sistema deveria continuar rodando após múltiplos Start")
	}

	// Para sistema
	ds.Stop()
	if ds.IsRunning() {
		t.Error("Sistema deveria estar parado após Stop")
	}

	// Stop múltiplos não devem causar problema
	ds.Stop()
	if ds.IsRunning() {
		t.Error("Sistema deveria continuar parado após múltiplos Stop")
	}
}

func TestDisseminationSystem_DisseminateDelta_NoNeighbors(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{}) // Sem vizinhos
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 3, 4, neighborGetter, tcpSender)

	ds.Start()
	delta := createTestDelta("test-drone")

	err := ds.DisseminateDelta(delta)
	if err != nil {
		t.Errorf("DisseminateDelta não deveria falhar sem vizinhos: %v", err)
	}

	// Verifica que nenhuma mensagem foi enviada
	sent := tcpSender.GetSentMessages()
	if len(sent) != 0 {
		t.Errorf("Nenhuma mensagem deveria ter sido enviada, obtido %d", len(sent))
	}
}

func TestDisseminationSystem_DisseminateDelta_WithNeighbors(t *testing.T) {
	neighbors := []string{"http://node1:8080", "http://node2:8080", "http://node3:8080"}
	neighborGetter := NewMockNeighborGetter(neighbors)
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 2, 3, neighborGetter, tcpSender)

	ds.Start()
	delta := createTestDelta("test-drone")

	err := ds.DisseminateDelta(delta)
	if err != nil {
		t.Errorf("DisseminateDelta não deveria falhar: %v", err)
	}

	// Verifica mensagens enviadas
	sent := tcpSender.GetSentMessages()
	if len(sent) != 2 { // fanout = 2
		t.Errorf("Esperado 2 mensagens enviadas (fanout), obtido %d", len(sent))
	}

	// Verifica que todas as mensagens têm TTL correto
	for i, msg := range sent {
		if msg.TTL != 3 {
			t.Errorf("Mensagem %d deveria ter TTL 3, obtido %d", i, msg.TTL)
		}
		if msg.SenderID != "test-drone" {
			t.Errorf("Mensagem %d deveria ter SenderID test-drone, obtido %s", i, msg.SenderID)
		}
		if msg.ID != delta.ID {
			t.Errorf("Mensagem %d deveria ter ID %s, obtido %s", i, delta.ID, msg.ID)
		}
	}

	// Verifica estatísticas
	stats := ds.GetStats()
	if sentCount, ok := stats["sent_count"].(int64); !ok || sentCount != 2 {
		t.Errorf("Esperado sent_count 2, obtido %v", stats["sent_count"])
	}
}

func TestDisseminationSystem_DisseminateDelta_NotRunning(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{"http://node1:8080"})
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 3, 4, neighborGetter, tcpSender)

	// Sistema parado
	delta := createTestDelta("test-drone")

	err := ds.DisseminateDelta(delta)
	if err != nil {
		t.Errorf("DisseminateDelta não deveria falhar com sistema parado: %v", err)
	}

	// Nenhuma mensagem deve ter sido enviada
	sent := tcpSender.GetSentMessages()
	if len(sent) != 0 {
		t.Errorf("Nenhuma mensagem deveria ter sido enviada com sistema parado, obtido %d", len(sent))
	}
}

func TestDisseminationSystem_ProcessReceivedDelta_Success(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{"http://node1:8080", "http://node2:8080"})
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 2, 4, neighborGetter, tcpSender)

	ds.Start()

	// Cria delta recebido
	deltaID := uuid.New()
	receivedDelta := DeltaMsg{
		ID:        deltaID,
		TTL:       2,
		SenderID:  "sender-drone",
		Timestamp: time.Now().UnixMilli(),
		Data: sensor.SensorDelta{
			ID:        deltaID,
			SensorID:  "remote-sensor",
			Value:     99.9,
			Timestamp: time.Now().UnixMilli(),
			DroneID:   "sender-drone",
		},
	}

	err := ds.ProcessReceivedDelta(receivedDelta)
	if err != nil {
		t.Errorf("ProcessReceivedDelta não deveria falhar: %v", err)
	}

	// Verifica que delta foi adicionado ao cache
	if !ds.cache.Contains(deltaID) {
		t.Error("Delta deveria estar no cache após processamento")
	}

	// Verifica que delta foi encaminhado com TTL decrementado
	sent := tcpSender.GetSentMessages()
	if len(sent) != 2 { // fanout = 2
		t.Errorf("Esperado 2 mensagens reenviadas, obtido %d", len(sent))
	}

	for i, msg := range sent {
		if msg.TTL != 1 { // TTL decrementado
			t.Errorf("Mensagem %d deveria ter TTL 1, obtido %d", i, msg.TTL)
		}
		if msg.SenderID != "test-drone" { // SenderID atualizado
			t.Errorf("Mensagem %d deveria ter SenderID test-drone, obtido %s", i, msg.SenderID)
		}
	}

	// Verifica estatísticas
	stats := ds.GetStats()
	if receivedCount, ok := stats["received_count"].(int64); !ok || receivedCount != 1 {
		t.Errorf("Esperado received_count 1, obtido %v", stats["received_count"])
	}
}

func TestDisseminationSystem_ProcessReceivedDelta_TTLZero(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{"http://node1:8080"})
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 3, 4, neighborGetter, tcpSender)

	ds.Start()

	// Delta com TTL=0
	deltaID := uuid.New()
	receivedDelta := DeltaMsg{
		ID:       deltaID,
		TTL:      0, // TTL zero
		SenderID: "sender-drone",
		Data:     createTestDelta("sender-drone"),
	}

	err := ds.ProcessReceivedDelta(receivedDelta)
	if err != nil {
		t.Errorf("ProcessReceivedDelta não deveria falhar com TTL=0: %v", err)
	}

	// Verifica que delta foi adicionado ao cache mesmo com TTL=0
	if !ds.cache.Contains(deltaID) {
		t.Error("Delta deveria estar no cache mesmo com TTL=0")
	}

	// Verifica que nenhuma mensagem foi reenviada
	sent := tcpSender.GetSentMessages()
	if len(sent) != 0 {
		t.Errorf("Nenhuma mensagem deveria ter sido reenviada com TTL=0, obtido %d", len(sent))
	}

	// Verifica estatísticas de dropped
	stats := ds.GetStats()
	if droppedCount, ok := stats["dropped_count"].(int64); !ok || droppedCount != 1 {
		t.Errorf("Esperado dropped_count 1, obtido %v", stats["dropped_count"])
	}
}

func TestDisseminationSystem_ProcessReceivedDelta_Duplicate(t *testing.T) {
	neighborGetter := NewMockNeighborGetter([]string{"http://node1:8080"})
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 3, 4, neighborGetter, tcpSender)

	ds.Start()

	deltaID := uuid.New()
	receivedDelta := DeltaMsg{
		ID:       deltaID,
		TTL:      2,
		SenderID: "sender-drone",
		Data:     createTestDelta("sender-drone"),
	}

	// Processa primeira vez
	err := ds.ProcessReceivedDelta(receivedDelta)
	if err != nil {
		t.Errorf("Primeiro ProcessReceivedDelta não deveria falhar: %v", err)
	}

	// Processa segunda vez (duplicata)
	err = ds.ProcessReceivedDelta(receivedDelta)
	if err != nil {
		t.Errorf("ProcessReceivedDelta duplicado não deveria falhar: %v", err)
	}

	// Verifica que apenas uma mensagem foi reenviada
	sent := tcpSender.GetSentMessages()
	if len(sent) != 1 {
		t.Errorf("Esperado 1 mensagem (só primeira), obtido %d", len(sent))
	}

	// Verifica estatísticas
	stats := ds.GetStats()
	if receivedCount, ok := stats["received_count"].(int64); !ok || receivedCount != 2 {
		t.Errorf("Esperado received_count 2, obtido %v", stats["received_count"])
	}
	if droppedCount, ok := stats["dropped_count"].(int64); !ok || droppedCount != 1 {
		t.Errorf("Esperado dropped_count 1 (duplicata), obtido %v", stats["dropped_count"])
	}
}

func TestDisseminationSystem_SendErrors(t *testing.T) {
	neighbors := []string{"http://good:8080", "http://bad:8080"}
	neighborGetter := NewMockNeighborGetter(neighbors)
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 2, 4, neighborGetter, tcpSender)

	// Configura TCP sender para falhar em uma URL específica
	tcpSender.SetShouldFail(true, "http://bad:8080")

	ds.Start()
	delta := createTestDelta("test-drone")

	err := ds.DisseminateDelta(delta)
	// Deve retornar erro, mas não falhar completamente
	if err == nil {
		t.Error("DisseminateDelta deveria retornar erro quando alguns envios falham")
	}

	// Verifica que pelo menos uma mensagem foi enviada (para o vizinho bom)
	sent := tcpSender.GetSentMessages()
	if len(sent) == 0 {
		t.Error("Pelo menos uma mensagem deveria ter sido enviada")
	}

	// Verifica estatísticas
	stats := ds.GetStats()
	sentCount, ok := stats["sent_count"].(int64)
	if !ok || sentCount == 0 {
		t.Errorf("Esperado sent_count > 0, obtido %v", stats["sent_count"])
	}
}

func TestDisseminationSystem_GetStats(t *testing.T) {
	neighbors := []string{"http://node1:8080", "http://node2:8080"}
	neighborGetter := NewMockNeighborGetter(neighbors)
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 3, 5, neighborGetter, tcpSender)

	stats := ds.GetStats()
	if stats == nil {
		t.Fatal("GetStats não deveria retornar nil")
	}

	// Verifica campos esperados
	expectedFields := []string{"running", "fanout", "default_ttl", "sent_count", "received_count", "dropped_count", "cache_size", "neighbor_count"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Stats deveria conter campo %s", field)
		}
	}

	// Verifica valores iniciais
	if running, ok := stats["running"].(bool); !ok || running {
		t.Errorf("Esperado running false, obtido %v", stats["running"])
	}

	if fanout, ok := stats["fanout"].(int); !ok || fanout != 3 {
		t.Errorf("Esperado fanout 3, obtido %v", stats["fanout"])
	}

	if neighborCount, ok := stats["neighbor_count"].(int); !ok || neighborCount != 2 {
		t.Errorf("Esperado neighbor_count 2, obtido %v", stats["neighbor_count"])
	}
}

func TestDisseminationSystem_ConcurrentOperations(t *testing.T) {
	neighbors := []string{"http://node1:8080", "http://node2:8080", "http://node3:8080"}
	neighborGetter := NewMockNeighborGetter(neighbors)
	tcpSender := NewMockTCPSender()
	ds := NewDisseminationSystem("test-drone", 2, 4, neighborGetter, tcpSender)

	ds.Start()

	var wg sync.WaitGroup
	numOperations := 50

	// Operações concorrentes de disseminação
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			delta := createTestDelta("test-drone")
			ds.DisseminateDelta(delta)
		}(i)
	}

	// Operações concorrentes de processamento
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			deltaID := uuid.New()
			msg := DeltaMsg{
				ID:       deltaID,
				TTL:      2,
				SenderID: "remote-drone",
				Data:     createTestDelta("remote-drone"),
			}
			ds.ProcessReceivedDelta(msg)
		}(i)
	}

	// Operações concorrentes de estatísticas
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			stats := ds.GetStats()
			if stats == nil {
				t.Error("GetStats não deveria retornar nil em operação concorrente")
			}
		}()
	}

	wg.Wait()

	// Verifica que sistema não está corrompido
	stats := ds.GetStats()
	if stats == nil {
		t.Fatal("Stats não deveria ser nil após operações concorrentes")
	}

	if running, ok := stats["running"].(bool); !ok || !running {
		t.Error("Sistema deveria estar running após operações concorrentes")
	}
}

func TestSelectRandomNeighbors(t *testing.T) {
	neighbors := []string{"node1", "node2", "node3", "node4", "node5"}

	// Teste com count menor que total
	selected := selectRandomNeighbors(neighbors, 3)
	if len(selected) != 3 {
		t.Errorf("Esperado 3 vizinhos selecionados, obtido %d", len(selected))
	}

	// Verifica que todos os selecionados estão na lista original
	for _, sel := range selected {
		found := false
		for _, orig := range neighbors {
			if sel == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Vizinho selecionado %s não está na lista original", sel)
		}
	}

	// Teste com count igual ao total
	selected = selectRandomNeighbors(neighbors, 5)
	if len(selected) != 5 {
		t.Errorf("Esperado 5 vizinhos selecionados, obtido %d", len(selected))
	}

	// Teste com count maior que total
	selected = selectRandomNeighbors(neighbors, 10)
	if len(selected) != 5 {
		t.Errorf("Esperado 5 vizinhos selecionados (máximo disponível), obtido %d", len(selected))
	}

	// Teste com lista vazia
	selected = selectRandomNeighbors([]string{}, 3)
	if len(selected) != 0 {
		t.Errorf("Esperado 0 vizinhos de lista vazia, obtido %d", len(selected))
	}

	// Teste que verifica randomização (executando várias vezes)
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		selected := selectRandomNeighbors(neighbors, 1)
		if len(selected) == 1 {
			selections[selected[0]]++
		}
	}

	// Deve ter selecionado pelo menos 2 vizinhos diferentes
	if len(selections) < 2 {
		t.Errorf("Seleção deveria ser aleatória, obtido apenas %d vizinhos únicos", len(selections))
	}
}
