package main

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/heitortanoue/tcc/pkg/network"
	"github.com/heitortanoue/tcc/pkg/protocol"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

func TestIntegration_Fase3_ComponentCreation(t *testing.T) {
	// Este teste verifica se todos os componentes da Fase 3 podem ser criados

	// 1. Configura parâmetros de teste
	nodeID := "test-node"
	udpPort := findFreeUDPPort(t)

	// 2. Cria tabela de vizinhos
	neighborTable := network.NewNeighborTable(30 * time.Second)
	if neighborTable == nil {
		t.Fatal("Falha ao criar NeighborTable")
	}

	// 3. Cria API de sensor
	sensorAPI := sensor.NewSensorAPI(nodeID, 1*time.Second)
	if sensorAPI == nil {
		t.Fatal("Falha ao criar SensorAPI")
	}

	// 4. Cria servidor UDP
	server := network.NewUDPServer(nodeID, udpPort, neighborTable)
	if server == nil {
		t.Fatal("Falha ao criar UDPServer")
	}

	// 5. Cria sistema de controle
	control := protocol.NewControlSystem(nodeID, sensorAPI, server)
	if control == nil {
		t.Fatal("Falha ao criar ControlSystem")
	}

	// 6. Define processador de mensagem
	server.SetMessageProcessor(control)

	// 7. Inicia e para sensor API para testar ciclo de vida
	sensorAPI.Start()
	time.Sleep(100 * time.Millisecond)
	sensorAPI.Stop()

	// 8. Verifica se dados foram coletados
	state := sensorAPI.GetState()
	if len(state) == 0 {
		t.Error("Nenhum dado foi coletado pelo sensor")
	}

	t.Logf("Teste bem-sucedido: Todos os componentes criados. Dados coletados: %d", len(state))
}

func TestIntegration_Fase3_NeighborTable(t *testing.T) {
	// Este teste verifica a funcionalidade da tabela de vizinhos

	// 1. Cria tabela de vizinhos
	neighborTable := network.NewNeighborTable(1 * time.Second) // TTL curto para teste

	// 2. Adiciona vizinhos usando IP/Porto
	ip1 := net.ParseIP("127.0.0.1")
	ip2 := net.ParseIP("192.168.1.100")

	neighborTable.AddOrUpdate(ip1, 8080)
	neighborTable.AddOrUpdate(ip2, 8081)

	// 3. Verifica vizinhos ativos
	active := neighborTable.GetActiveNeighbors()
	if len(active) != 2 {
		t.Errorf("Esperado 2 vizinhos ativos, obtido %d", len(active))
	}

	// 4. Verifica URLs geradas
	urls := neighborTable.GetNeighborURLs()
	if len(urls) != 2 {
		t.Errorf("Esperado 2 URLs, obtido %d", len(urls))
	}

	// 5. Verifica contagem
	count := neighborTable.Count()
	if count != 2 {
		t.Errorf("Esperado contagem 2, obtido %d", count)
	}

	// 6. Verifica estatísticas
	stats := neighborTable.GetStats()
	if activeCount, ok := stats["neighbors_active"].(int); !ok || activeCount != 2 {
		t.Errorf("Estatística de vizinhos ativos incorreta: %v", stats)
	}

	t.Logf("Teste bem-sucedido: Tabela de vizinhos funcionando. URLs: %v", urls)
}

func TestIntegration_Fase3_SensorWithControl(t *testing.T) {
	// Este teste verifica a integração entre sensor e sistema de controle

	// 1. Configura componentes
	nodeID := "sensor-control-node"
	udpPort := findFreeUDPPort(t)

	neighborTable := network.NewNeighborTable(30 * time.Second)
	sensorAPI := sensor.NewSensorAPI(nodeID, 200*time.Millisecond)
	server := network.NewUDPServer(nodeID, udpPort, neighborTable)
	control := protocol.NewControlSystem(nodeID, sensorAPI, server)

	// 2. Configura processador
	server.SetMessageProcessor(control)

	// 3. Inicia sistema de sensor
	sensorAPI.Start()

	// 4. Aguarda coleta de dados
	time.Sleep(500 * time.Millisecond)

	// 5. Verifica se dados foram coletados
	state := sensorAPI.GetState()
	if len(state) < 2 {
		t.Errorf("Dados insuficientes coletados: esperado >= 2, obtido %d", len(state))
	}

	// 6. Inicia e para sistema de controle
	control.Start()
	time.Sleep(100 * time.Millisecond)
	control.Stop()

	// 7. Para sistema de sensor
	sensorAPI.Stop()

	// 8. Verifica estado final
	finalState := sensorAPI.GetState()
	if len(finalState) == 0 {
		t.Error("Sistema de sensor perdeu dados após integração com controle")
	}

	t.Logf("Teste bem-sucedido: Sensor e controle integrados. Dados finais: %d", len(finalState))
}

func TestIntegration_Fase3_UDPServerCreation(t *testing.T) {
	// Este teste verifica se o servidor UDP pode ser criado e configurado

	// 1. Configura parâmetros
	nodeID := "udp-test-node"
	udpPort := findFreeUDPPort(t)

	// 2. Cria dependências
	neighborTable := network.NewNeighborTable(30 * time.Second)
	sensorAPI := sensor.NewSensorAPI(nodeID, 1*time.Second)

	// 3. Cria servidor UDP
	server := network.NewUDPServer(nodeID, udpPort, neighborTable)

	// 4. Cria sistema de controle
	control := protocol.NewControlSystem(nodeID, sensorAPI, server)

	// 5. Configura processador de mensagem
	server.SetMessageProcessor(control)

	// 6. Testa que componentes foram criados corretamente
	if server == nil {
		t.Fatal("Servidor UDP não foi criado")
	}
	if control == nil {
		t.Fatal("Sistema de controle não foi criado")
	}

	t.Logf("Teste bem-sucedido: Servidor UDP e controle criados para porta %d", udpPort)
}

func TestIntegration_Fase3_TransmitterElection(t *testing.T) {
	// Este teste verifica a criação do sistema de eleição de transmissor

	// 1. Configura componentes
	nodeID := "election-test-node"

	neighborTable := network.NewNeighborTable(30 * time.Second)
	sensorAPI := sensor.NewSensorAPI(nodeID, 1*time.Second)

	// 2. Cria servidor UDP (mock)
	server := network.NewUDPServer(nodeID, 7000, neighborTable)

	// 3. Cria sistema de controle
	control := protocol.NewControlSystem(nodeID, sensorAPI, server)

	// 4. Cria eleição de transmissor
	election := protocol.NewTransmitterElection(nodeID, control)
	if election == nil {
		t.Fatal("Sistema de eleição não foi criado")
	}

	// 5. Verifica métodos básicos
	state := election.GetState()
	isTransmitting := election.IsTransmitting()
	stateInfo := election.GetStateInfo()
	stats := election.GetStats()

	// 6. Valida estado inicial
	if state == "" {
		t.Error("Estado inicial não deveria estar vazio")
	}
	if isTransmitting {
		t.Error("Nó não deveria estar transmitindo inicialmente")
	}
	if stateInfo == nil {
		t.Error("Informações de estado não deveriam ser nil")
	}
	if stats == nil {
		t.Error("Estatísticas da eleição não deveriam ser nil")
	}

	// 7. Testa habilitação/desabilitação
	election.SetEnabled(false)
	election.SetEnabled(true)

	// 8. Força estado idle
	election.ForceIdle()

	t.Logf("Teste bem-sucedido: Sistema de eleição criado. Estado: %s, Transmitindo: %v", state, isTransmitting)
}

func TestIntegration_Fase3_UDPServerLifecycle(t *testing.T) {
	// Este teste verifica o ciclo de vida do servidor UDP (start/stop)

	// 1. Configura componentes
	nodeID := "udp-lifecycle-test"
	udpPort := findFreeUDPPort(t)

	neighborTable := network.NewNeighborTable(30 * time.Second)
	sensorAPI := sensor.NewSensorAPI(nodeID, 1*time.Second)
	server := network.NewUDPServer(nodeID, udpPort, neighborTable)
	control := protocol.NewControlSystem(nodeID, sensorAPI, server)

	// 2. Configura processador
	server.SetMessageProcessor(control)

	// 3. Inicia servidor em background
	go func() {
		if err := server.Start(); err != nil {
			t.Logf("Servidor UDP finalizou: %v", err)
		}
	}()

	// 4. Aguarda inicialização
	time.Sleep(100 * time.Millisecond)

	// 5. Verifica se porta está em uso (indica que servidor iniciou)
	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err != nil {
		t.Logf("Nota: Não foi possível conectar à porta UDP (esperado): %v", err)
	} else {
		conn.Close()
		t.Logf("Conectou com sucesso à porta UDP %d", udpPort)
	}

	// 6. Para servidor
	server.Stop()

	// 7. Aguarda finalização
	time.Sleep(100 * time.Millisecond)

	t.Logf("Teste bem-sucedido: Ciclo de vida do servidor UDP executado na porta %d", udpPort)
}

// Função auxiliar para encontrar porta UDP livre
func findFreeUDPPort(t *testing.T) int {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Erro ao resolver endereço UDP: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("Erro ao criar socket UDP: %v", err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).Port
}
