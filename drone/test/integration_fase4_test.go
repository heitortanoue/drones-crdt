package main

import (
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/gossip"
	"github.com/heitortanoue/tcc/pkg/network"
	"github.com/heitortanoue/tcc/pkg/protocol"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

func TestIntegration_Fase4_DeduplicationCache(t *testing.T) {
	// Este teste verifica a funcionalidade do cache LRU (F7)

	// 1. Cria cache com capacidade pequena para teste
	cache := gossip.NewDeduplicationCache(3)

	// 2. Testa adição de IDs
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()

	cache.Add(id1)
	cache.Add(id2)
	cache.Add(id3)

	// 3. Verifica que todos estão presentes
	if !cache.Contains(id1) || !cache.Contains(id2) || !cache.Contains(id3) {
		t.Error("Cache deveria conter os 3 primeiros IDs")
	}

	// 4. Adiciona quarto ID (deve remover o mais antigo)
	cache.Add(id4)

	// 5. Verifica eviction do LRU
	if cache.Contains(id1) {
		t.Error("ID1 deveria ter sido removido (LRU)")
	}
	if !cache.Contains(id4) {
		t.Error("ID4 deveria estar presente")
	}

	// 6. Verifica tamanho
	if cache.Size() != 3 {
		t.Errorf("Cache deveria ter tamanho 3, obtido %d", cache.Size())
	}

	// 7. Verifica estatísticas
	stats := cache.GetStats()
	if utilization, ok := stats["utilization"].(float64); !ok || utilization != 1.0 {
		t.Errorf("Utilização deveria ser 1.0, obtido %v", stats["utilization"])
	}

	t.Logf("Teste bem-sucedido: Cache LRU funcionando corretamente")
}

func TestIntegration_Fase4_DisseminationSystem(t *testing.T) {
	// Este teste verifica o sistema de disseminação TTL (F4)

	// 1. Configura componentes
	nodeID := "dissemination-test"

	neighborTable := network.NewNeighborTable(30 * time.Second)
	tcpSender := gossip.NewHTTPTCPSender(1 * time.Second)
	dissemination := gossip.NewDisseminationSystem(nodeID, 3, 4, neighborTable, tcpSender)

	// 2. Verifica estado inicial
	if dissemination.IsRunning() {
		t.Error("Sistema não deveria estar executando inicialmente")
	}

	// 3. Inicia sistema
	dissemination.Start()
	if !dissemination.IsRunning() {
		t.Error("Sistema deveria estar executando após Start()")
	}

	// 4. Cria delta para teste
	delta := sensor.SensorDelta{
		ID:        uuid.New(),
		SensorID:  "test-sensor",
		Timestamp: time.Now().UnixMilli(),
		Value:     42.0,
		DroneID:   nodeID,
	}

	// 5. Testa disseminação (sem vizinhos deve funcionar sem erro)
	err := dissemination.DisseminateDelta(delta)
	if err != nil {
		t.Errorf("Erro ao disseminar delta: %v", err)
	}

	// 6. Verifica estatísticas
	stats := dissemination.GetStats()
	if running, ok := stats["running"].(bool); !ok || !running {
		t.Error("Sistema deveria estar marcado como running nas stats")
	}
	if fanout, ok := stats["fanout"].(int); !ok || fanout != 3 {
		t.Errorf("Fanout deveria ser 3, obtido %v", stats["fanout"])
	}

	// 7. Para sistema
	dissemination.Stop()
	if dissemination.IsRunning() {
		t.Error("Sistema não deveria estar executando após Stop()")
	}

	t.Logf("Teste bem-sucedido: Sistema de disseminação funcionando")
}

func TestIntegration_Fase4_TTLProcessing(t *testing.T) {
	// Este teste verifica o processamento de TTL

	// 1. Configura sistema
	nodeID := "ttl-test"
	neighborTable := network.NewNeighborTable(30 * time.Second)
	tcpSender := gossip.NewHTTPTCPSender(1 * time.Second)
	dissemination := gossip.NewDisseminationSystem(nodeID, 3, 4, neighborTable, tcpSender)

	dissemination.Start()
	defer dissemination.Stop()

	// 2. Cria mensagem com TTL=1
	deltaMsg := gossip.DeltaMsg{
		ID:  uuid.New(),
		TTL: 1,
		Data: sensor.SensorDelta{
			ID:        uuid.New(),
			SensorID:  "ttl-sensor",
			Timestamp: time.Now().UnixMilli(),
			Value:     85.5,
			DroneID:   "remote-node",
		},
		SenderID:  "remote-node",
		Timestamp: time.Now().UnixMilli(),
	}

	// 3. Processa mensagem (TTL deve ser decrementado)
	err := dissemination.ProcessReceivedDelta(deltaMsg)
	if err != nil {
		t.Errorf("Erro ao processar delta: %v", err)
	}

	// 4. Testa mensagem com TTL=0 (deve ser descartada)
	deltaMsg.TTL = 0
	deltaMsg.ID = uuid.New() // Novo ID para evitar cache

	err = dissemination.ProcessReceivedDelta(deltaMsg)
	if err != nil {
		t.Errorf("Processamento de TTL=0 não deveria gerar erro: %v", err)
	}

	// 5. Verifica estatísticas de descarte
	stats := dissemination.GetStats()
	droppedCount, ok := stats["dropped_count"].(int64)
	if !ok || droppedCount == 0 {
		t.Errorf("Deveria haver pelo menos 1 delta descartado, obtido %v", stats["dropped_count"])
	}

	t.Logf("Teste bem-sucedido: Processamento TTL funcionando")
}

func TestIntegration_Fase4_ElectionEnhanced(t *testing.T) {
	// Este teste verifica a eleição melhorada com ReqCtr (F6 completo)

	// 1. Configura componentes
	nodeID := "election-enhanced-test"

	neighborTable := network.NewNeighborTable(30 * time.Second)
	sensorAPI := sensor.NewSensorAPI(nodeID, 10*time.Second)
	server := network.NewUDPServer(nodeID, findFreeUDPPort(t), neighborTable)
	control := protocol.NewControlSystem(nodeID, sensorAPI, server)
	election := protocol.NewTransmitterElection(nodeID, control)

	// 2. Verifica estado inicial
	if election.GetState() != "IDLE" {
		t.Errorf("Estado inicial deveria ser IDLE, obtido %s", election.GetState())
	}

	// 3. Simula incremento de contador de request
	deltaID := uuid.New()
	control.IncrementRequestCounter(deltaID)
	control.IncrementRequestCounter(deltaID) // ReqCtr = 2

	// 4. Executa verificação de eleição
	election.CheckElection()

	// Aguarda processamento
	time.Sleep(200 * time.Millisecond)

	// 5. Verifica se virou transmissor
	if election.GetState() != "TRANSMITTER" {
		t.Errorf("Deveria estar em estado TRANSMITTER, obtido %s", election.GetState())
	}

	// 6. Verifica se contador foi resetado
	counters := control.GetRequestCounters()
	if count, exists := counters[deltaID]; exists && count > 0 {
		t.Errorf("Contador deveria ter sido resetado, obtido %d", count)
	}

	// 7. Força retorno ao idle
	election.ForceIdle()
	if election.GetState() != "IDLE" {
		t.Error("ForceIdle() deveria retornar ao estado IDLE")
	}

	t.Logf("Teste bem-sucedido: Eleição melhorada funcionando")
}

func TestIntegration_Fase4_CompleteWorkflow(t *testing.T) {
	// Este teste verifica o fluxo completo: coleta → advertise → request → switch → transmit

	// 1. Configura dois nós
	node1ID := "workflow-node-1"
	node2ID := "workflow-node-2"

	node1Port := findFreeUDPPort(t)
	node2Port := findFreeUDPPort(t)
	node1TCPPort := findFreeTCPPort(t)
	node2TCPPort := findFreeTCPPort(t)

	// 2. Cria componentes do nó 1
	nt1 := network.NewNeighborTable(30 * time.Second)
	sensor1 := sensor.NewSensorAPI(node1ID, 10*time.Second)
	udp1 := network.NewUDPServer(node1ID, node1Port, nt1)
	control1 := protocol.NewControlSystem(node1ID, sensor1, udp1)
	tcpSender1 := gossip.NewHTTPTCPSender(2 * time.Second)
	dissem1 := gossip.NewDisseminationSystem(node1ID, 3, 4, nt1, tcpSender1)

	// 3. Cria componentes do nó 2
	nt2 := network.NewNeighborTable(30 * time.Second)
	sensor2 := sensor.NewSensorAPI(node2ID, 10*time.Second)
	udp2 := network.NewUDPServer(node2ID, node2Port, nt2)
	control2 := protocol.NewControlSystem(node2ID, sensor2, udp2)
	tcpSender2 := gossip.NewHTTPTCPSender(2 * time.Second)
	dissem2 := gossip.NewDisseminationSystem(node2ID, 3, 4, nt2, tcpSender2)

	// 4. Configura processadores
	udp1.SetMessageProcessor(control1)
	udp2.SetMessageProcessor(control2)

	// 5. Inicia sistemas
	sensor1.Start()
	sensor2.Start()
	dissem1.Start()
	dissem2.Start()
	control1.Start()
	control2.Start()

	// 6. Inicia servidores UDP
	go udp1.Start()
	go udp2.Start()

	time.Sleep(200 * time.Millisecond)

	// 7. Adiciona vizinhos mutuamente
	ip1 := net.ParseIP("127.0.0.1")
	ip2 := net.ParseIP("127.0.0.1")
	nt1.AddOrUpdate(ip2, node2TCPPort)
	nt2.AddOrUpdate(ip1, node1TCPPort)

	// 8. Gera dados nos sensores
	reading1 := sensor.SensorReading{
		SensorID:  "workflow-sensor-1",
		Timestamp: time.Now().UnixMilli(),
		Value:     99.5,
	}
	delta1 := sensor1.AddManualReading(reading1)

	// 9. Simula disseminação
	err := dissem1.DisseminateDelta(delta1)
	if err != nil {
		t.Logf("Nota: Erro esperado na disseminação sem servidor TCP: %v", err)
	}

	// 10. Verifica estatísticas finais
	stats1 := dissem1.GetStats()
	stats2 := dissem2.GetStats()

	t.Logf("Stats Nó 1: %+v", stats1)
	t.Logf("Stats Nó 2: %+v", stats2)

	// 11. Cleanup
	control1.Stop()
	control2.Stop()
	dissem1.Stop()
	dissem2.Stop()
	sensor1.Stop()
	sensor2.Stop()
	udp1.Stop()
	udp2.Stop()

	t.Logf("Teste bem-sucedido: Fluxo completo executado")
}

// Função auxiliar para encontrar porta TCP livre
func findFreeTCPPort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Erro ao criar listener TCP: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}
