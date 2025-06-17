package network

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"
)

// MockMessageProcessor implementa MessageProcessor para testes
type MockMessageProcessor struct {
	receivedMessages []ReceivedMessage
	mutex            sync.Mutex
}

type ReceivedMessage struct {
	Data     []byte
	SenderIP string
}

func NewMockMessageProcessor() *MockMessageProcessor {
	return &MockMessageProcessor{
		receivedMessages: make([]ReceivedMessage, 0),
	}
}

func (m *MockMessageProcessor) ProcessMessage(data []byte, senderIP string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.receivedMessages = append(m.receivedMessages, ReceivedMessage{
		Data:     data,
		SenderIP: senderIP,
	})
}

func (m *MockMessageProcessor) GetReceivedMessages() []ReceivedMessage {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	result := make([]ReceivedMessage, len(m.receivedMessages))
	copy(result, m.receivedMessages)
	return result
}

func (m *MockMessageProcessor) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.receivedMessages = make([]ReceivedMessage, 0)
}

// Função auxiliar para encontrar uma porta UDP livre
func findFreeUDPPort() int {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		return 0
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).Port
}

func TestUDPServer_NewUDPServer(t *testing.T) {
	droneID := "test-udp-drone"
	port := 7000
	nt := NewNeighborTable(10 * time.Second)

	server := NewUDPServer(droneID, port, nt)

	if server == nil {
		t.Fatal("NewUDPServer não deveria retornar nil")
	}

	if server.droneID != droneID {
		t.Errorf("DroneID esperado %s, obtido %s", droneID, server.droneID)
	}

	if server.port != port {
		t.Errorf("Porta esperada %d, obtida %d", port, server.port)
	}

	if server.neighborTable != nt {
		t.Error("NeighborTable não foi configurada corretamente")
	}

	if server.running {
		t.Error("Servidor não deveria estar running na criação")
	}
}

func TestUDPServer_SetMessageProcessor(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)
	server := NewUDPServer("test", 7001, nt)
	processor := NewMockMessageProcessor()

	server.SetMessageProcessor(processor)

	if server.messageProcessor != processor {
		t.Error("MessageProcessor não foi configurado corretamente")
	}
}

func TestUDPServer_StartStop(t *testing.T) {
	port := findFreeUDPPort()
	if port == 0 {
		t.Fatal("Não foi possível encontrar porta UDP livre")
	}

	nt := NewNeighborTable(10 * time.Second)
	server := NewUDPServer("start-stop-test", port, nt)

	// Teste Start
	err := server.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar servidor: %v", err)
	}

	if !server.running {
		t.Error("Servidor deveria estar running após Start")
	}

	if server.conn == nil {
		t.Error("Conexão UDP deveria estar estabelecida")
	}

	stats := server.GetStats()
	if stats["running"] != true {
		t.Error("Stats deveriam mostrar servidor running")
	}

	// Teste Stop
	err = server.Stop()
	if err != nil {
		t.Fatalf("Erro ao parar servidor: %v", err)
	}

	if server.running {
		t.Error("Servidor não deveria estar running após Stop")
	}
}

func TestUDPServer_SendPacket(t *testing.T) {
	// Configura servidor receptor
	receiverPort := findFreeUDPPort()
	receiverNT := NewNeighborTable(10 * time.Second)
	receiver := NewUDPServer("receiver", receiverPort, receiverNT)
	processor := NewMockMessageProcessor()
	receiver.SetMessageProcessor(processor)

	err := receiver.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar receptor: %v", err)
	}
	defer receiver.Stop()

	// Configura servidor remetente
	senderPort := findFreeUDPPort()
	senderNT := NewNeighborTable(10 * time.Second)
	sender := NewUDPServer("sender", senderPort, senderNT)

	err = sender.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar remetente: %v", err)
	}
	defer sender.Stop()

	// Envia pacote
	testData := []byte("teste de mensagem UDP")
	targetIP := net.ParseIP("127.0.0.1")

	err = sender.SendPacket(testData, targetIP, receiverPort)
	if err != nil {
		t.Fatalf("Erro ao enviar pacote: %v", err)
	}

	// Aguarda processamento
	time.Sleep(100 * time.Millisecond)

	// Verifica se mensagem foi recebida
	messages := processor.GetReceivedMessages()
	if len(messages) != 1 {
		t.Fatalf("Esperado 1 mensagem recebida, obtido %d", len(messages))
	}

	if string(messages[0].Data) != string(testData) {
		t.Errorf("Dados esperados %s, obtido %s", string(testData), string(messages[0].Data))
	}

	if messages[0].SenderIP != "127.0.0.1" {
		t.Errorf("IP do remetente esperado 127.0.0.1, obtido %s", messages[0].SenderIP)
	}
}

func TestUDPServer_SendTo(t *testing.T) {
	port := findFreeUDPPort()
	nt := NewNeighborTable(10 * time.Second)
	server := NewUDPServer("sendto-test", port, nt)

	err := server.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	defer server.Stop()

	// Testa SendTo com IP válido
	testData := []byte("teste SendTo")
	err = server.SendTo(testData, "127.0.0.1", 9999)
	if err != nil {
		t.Errorf("SendTo não deveria retornar erro para IP válido: %v", err)
	}

	// Testa SendTo com IP inválido
	err = server.SendTo(testData, "ip-inválido", 9999)
	if err == nil {
		t.Error("SendTo deveria retornar erro para IP inválido")
	}
}

func TestUDPServer_Broadcast(t *testing.T) {
	port := findFreeUDPPort()
	nt := NewNeighborTable(10 * time.Second)
	server := NewUDPServer("broadcast-test", port, nt)

	err := server.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	defer server.Stop()

	// Adiciona alguns vizinhos à tabela
	nt.AddOrUpdate(net.ParseIP("192.168.1.10"), 8080)
	nt.AddOrUpdate(net.ParseIP("192.168.1.20"), 8081)
	nt.AddOrUpdate(net.ParseIP("10.0.0.1"), 8082)

	// Teste broadcast (não deveria causar erro mesmo que os destinos não existam)
	testData := []byte("broadcast message")
	server.Broadcast(testData) // Não verifica erro pois destinos podem não existir

	// Verifica que o método executa sem panic
	// Em ambiente real, seria necessário interceptar logs ou configurar servidores receptores
}

func TestUDPServer_NeighborTableIntegration(t *testing.T) {
	// Configura dois servidores para teste de descoberta
	port1 := findFreeUDPPort()
	port2 := findFreeUDPPort()

	nt1 := NewNeighborTable(10 * time.Second)
	nt2 := NewNeighborTable(10 * time.Second)

	server1 := NewUDPServer("server1", port1, nt1)
	server2 := NewUDPServer("server2", port2, nt2)

	err := server1.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar server1: %v", err)
	}
	defer server1.Stop()

	err = server2.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar server2: %v", err)
	}
	defer server2.Stop()

	// Server1 envia mensagem para server2
	testData := []byte(`{"type": "test", "message": "hello"}`)
	err = server1.SendTo(testData, "127.0.0.1", port2)
	if err != nil {
		t.Fatalf("Erro ao enviar mensagem: %v", err)
	}

	// Aguarda processamento
	time.Sleep(100 * time.Millisecond)

	// Verifica se server2 adicionou server1 como vizinho
	neighbors := nt2.GetActiveNeighbors()
	if len(neighbors) != 1 {
		t.Fatalf("Server2 deveria ter 1 vizinho, obtido %d", len(neighbors))
	}

	neighbor := neighbors[0]
	if !neighbor.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("IP do vizinho deveria ser 127.0.0.1, obtido %v", neighbor.IP)
	}

	if neighbor.Port != 8080 {
		t.Errorf("Porta padrão deveria ser 8080, obtido %d", neighbor.Port)
	}
}

func TestUDPServer_GetStats(t *testing.T) {
	droneID := "stats-test"
	port := 7777
	nt := NewNeighborTable(5 * time.Second)
	server := NewUDPServer(droneID, port, nt)

	stats := server.GetStats()

	expectedFields := []string{"udp_port", "running", "drone_id"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Campo %s deveria existir em GetStats", field)
		}
	}

	if stats["udp_port"] != port {
		t.Errorf("udp_port esperado %d, obtido %v", port, stats["udp_port"])
	}

	if stats["running"] != false {
		t.Errorf("running esperado false, obtido %v", stats["running"])
	}

	if stats["drone_id"] != droneID {
		t.Errorf("drone_id esperado %s, obtido %v", droneID, stats["drone_id"])
	}

	// Testa stats com servidor running
	freePort := findFreeUDPPort()
	server2 := NewUDPServer("running-test", freePort, nt)
	server2.Start()
	defer server2.Stop()

	runningStats := server2.GetStats()
	if runningStats["running"] != true {
		t.Error("Stats deveriam mostrar running=true quando servidor está ativo")
	}
}

func TestUDPServer_MessageProcessing_WithoutProcessor(t *testing.T) {
	port := findFreeUDPPort()
	nt := NewNeighborTable(10 * time.Second)
	server := NewUDPServer("no-processor-test", port, nt)

	err := server.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	defer server.Stop()

	// Envia mensagem JSON válida para si mesmo
	testMessage := map[string]interface{}{
		"type": "test",
		"data": "exemplo",
	}

	jsonData, _ := json.Marshal(testMessage)
	err = server.SendTo(jsonData, "127.0.0.1", port)
	if err != nil {
		t.Fatalf("Erro ao enviar mensagem: %v", err)
	}

	// Aguarda processamento
	time.Sleep(100 * time.Millisecond)

	// Não deveria causar panic mesmo sem processador
	// O teste passa se não há crash
}

func TestUDPServer_ConcurrentOperations(t *testing.T) {
	port := findFreeUDPPort()
	nt := NewNeighborTable(10 * time.Second)
	server := NewUDPServer("concurrent-test", port, nt)
	processor := NewMockMessageProcessor()
	server.SetMessageProcessor(processor)

	err := server.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar servidor: %v", err)
	}
	defer server.Stop()

	const numGoroutines = 5
	const numMessages = 10

	var wg sync.WaitGroup

	// Múltiplas goroutines enviando mensagens concorrentemente
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numMessages; j++ {
				message := map[string]interface{}{
					"sender": id,
					"seq":    j,
					"data":   "concurrent test",
				}

				jsonData, _ := json.Marshal(message)
				server.SendTo(jsonData, "127.0.0.1", port)

				// Pequeno delay para evitar saturação
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Aguarda processamento de todas as mensagens
	time.Sleep(500 * time.Millisecond)

	// Verifica que mensagens foram recebidas
	messages := processor.GetReceivedMessages()
	expectedTotal := numGoroutines * numMessages

	if len(messages) != expectedTotal {
		t.Errorf("Esperado %d mensagens, obtido %d", expectedTotal, len(messages))
	}

	// Verifica que servidor ainda está funcional
	stats := server.GetStats()
	if stats["running"] != true {
		t.Error("Servidor deveria ainda estar running após operações concorrentes")
	}
}

func TestUDPServer_ErrorHandling(t *testing.T) {
	nt := NewNeighborTable(5 * time.Second)

	// Testa inicialização em porta ocupada
	port := findFreeUDPPort()
	server1 := NewUDPServer("error-test-1", port, nt)
	server2 := NewUDPServer("error-test-2", port, nt)

	err := server1.Start()
	if err != nil {
		t.Fatalf("Erro ao iniciar primeiro servidor: %v", err)
	}
	defer server1.Stop()

	// Segundo servidor na mesma porta deveria falhar
	err = server2.Start()
	if err == nil {
		t.Error("Segundo servidor deveria falhar ao usar porta ocupada")
		server2.Stop()
	}

	// Testa operações em servidor não iniciado
	uninitializedServer := NewUDPServer("uninitialized", 9999, nt)

	err = uninitializedServer.SendPacket([]byte("test"), net.ParseIP("127.0.0.1"), 8080)
	if err == nil {
		t.Error("SendPacket deveria falhar em servidor não iniciado")
	}

	// Testa Stop em servidor não iniciado
	err = uninitializedServer.Stop()
	if err != nil {
		t.Errorf("Stop não deveria retornar erro para servidor não iniciado: %v", err)
	}
}
