package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/heitortanoue/tcc/internal/config"
	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/gossip"
	"github.com/heitortanoue/tcc/pkg/network"
	"github.com/heitortanoue/tcc/pkg/protocol"
	"github.com/heitortanoue/tcc/pkg/sensor"
	"github.com/heitortanoue/tcc/pkg/state"
)

var startTime = time.Now() // Para cálculo de uptime

func main() {
	// Flags da linha de comando (novos requisitos da Fase 1)
	var (
		droneID   = flag.String("id", "drone-1", "ID único deste drone")
		sampleSec = flag.Int("sample-sec", 10, "Intervalo de coleta de sensor em segundos")
		fanout    = flag.Int("fanout", 3, "Número de vizinhos para gossip")
		ttl       = flag.Int("ttl", 4, "TTL inicial para mensagens")
		udpPort   = flag.Int("udp-port", 7000, "Porta UDP para controle")
		tcpPort   = flag.Int("tcp-port", 8080, "Porta TCP para dados")
		bindAddr  = flag.String("bind", "0.0.0.0", "Endereço para bind")
		showUsage = flag.Bool("help", false, "Mostra ajuda de uso")
	)
	flag.Parse()

	if *showUsage {
		printUsage()
		return
	}

	// Cria configuração do drone
	cfg := config.DefaultConfig()
	cfg.DroneID = *droneID
	cfg.SampleInterval = time.Duration(*sampleSec) * time.Second
	cfg.Fanout = *fanout
	cfg.TTL = *ttl
	cfg.UDPPort = *udpPort
	cfg.TCPPort = *tcpPort
	cfg.BindAddr = *bindAddr

	// Cria tabela de vizinhos
	neighborTable := network.NewNeighborTable(cfg.NeighborTimeout)

	// Inicia estado global
	state.InitGlobalState(cfg.DroneID)

	// Cria sistema de sensores (Fase 2: F1 + F2)
	sensorAPI := sensor.NewFireSensor(cfg.DroneID, cfg.SampleInterval)

	// Cria servidores UDP e TCP
	udpServer := network.NewUDPServer(cfg.DroneID, cfg.UDPPort, neighborTable)
	tcpServer := network.NewTCPServer(cfg.DroneID, cfg.TCPPort)

	// Cria sistema de controle (Fase 3: F3 + base F6)
	controlSystem := protocol.NewControlSystem(cfg.DroneID, sensorAPI, udpServer)

	// Cria sistema de disseminação TTL
	tcpSender := gossip.NewHTTPTCPSender(5 * time.Second)
	disseminationSystem := gossip.NewDisseminationSystem(cfg.DroneID, cfg.Fanout, cfg.TTL, neighborTable, tcpSender)

	// Integra handlers do sensor no TCP server
	tcpServer.SensorHandler = createSensorHandler(sensorAPI, disseminationSystem)
	tcpServer.DeltaHandler = createDeltaHandler(sensorAPI, disseminationSystem)
	tcpServer.StateHandler = createStateHandler(sensorAPI)
	tcpServer.StatsHandler = createStatsHandler(sensorAPI, neighborTable, controlSystem, disseminationSystem)

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nRecebido sinal de interrupção, desligando...")

		fmt.Println("Parando sistema de controle...")
		controlSystem.Stop()

		fmt.Println("Parando sistema de disseminação...")
		disseminationSystem.Stop()

		fmt.Println("Parando coleta de sensores...")
		sensorAPI.Stop()

		fmt.Println("Parando servidor UDP...")
		if err := udpServer.Stop(); err != nil {
			fmt.Printf("Erro ao parar UDP: %v\n", err)
		}

		fmt.Println("Parando servidor TCP...")
		if err := tcpServer.Stop(); err != nil {
			fmt.Printf("Erro ao parar TCP: %v\n", err)
		}

		os.Exit(0)
	}()

	// Mostra informações de inicialização
	fmt.Printf("=== Drone %s ===\n", cfg.DroneID)
	fmt.Printf("UDP (controle): %s:%d\n", cfg.BindAddr, cfg.UDPPort)
	fmt.Printf("TCP (dados): http://%s:%d\n", cfg.BindAddr, cfg.TCPPort)
	fmt.Printf("Coleta: a cada %v\n", cfg.SampleInterval)
	fmt.Printf("Gossip: fanout=%d, ttl=%d\n", cfg.Fanout, cfg.TTL)
	fmt.Printf("Iniciando...\n\n")
	// Inicia coleta automática de sensores (Fase 2: F1)
	sensorAPI.Start()

	// Inicia sistema de controle (Fase 3: F3)
	controlSystem.Start()

	// Inicia sistema de disseminação (Fase 4: F4 + F7)
	disseminationSystem.Start()

	// Inicia servidor UDP
	if err := udpServer.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor UDP: %v", err)
	}

	// Inicia servidor TCP (bloqueia até terminar)
	if err := tcpServer.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor TCP: %v", err)
	}
}

// printUsage mostra exemplos de uso
func printUsage() {
	fmt.Fprintf(os.Stderr, `
=== Drone Sistema de Sensores ===

USAGE:
  %s [opções]

EXAMPLES:
  # Drone básico
  %s -id=drone-1 -sample-sec=10

  # Drone com configuração customizada
  %s -id=drone-2 -sample-sec=5 -fanout=2 -ttl=3

  # Drone com portas específicas
  %s -id=drone-3 -udp-port=7001 -tcp-port=8081

OPTIONS:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])

	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, `
ENDPOINTS (TCP):
  GET  /health     - Status do drone
  POST /sensor     - Adiciona leitura de sensor (Fase 2)
  POST /delta      - Recebe deltas de outros drones (Fase 2)
  GET  /state      - Estado atual do CRDT (Fase 2)
  GET  /stats      - Estatísticas do drone (Fase 5)
  POST /neighbor   - Gerencia vizinhos (Fase 1)

PROTOCOLS:
  - UDP %d: Canal de controle (Hello)
  - TCP %d: Canal de dados (HTTP REST API)
  - Coleta automática de sensores a cada -sample-sec segundos
  - Descoberta de vizinhos via pacotes UDP
  - TTL gossip com fan-out configurável
`, 7000, 8080)
}

// Handlers HTTP para integração com sistema de sensores

// createSensorHandler cria handler para POST /sensor
//  O sensor vai enviar dados por aqui para o drone processar
func createSensorHandler(sensorAPI *sensor.FireSensor, dissemination *gossip.DisseminationSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var reading sensor.FireReading
		if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		var cell crdt.Cell
		cell.X = reading.X
		cell.Y = reading.Y
		var meta crdt.FireMeta
		meta.Timestamp = reading.Timestamp
		meta.Confidence = reading.Confidence
		meta.Temperature = reading.Temperature

		// Adiciona a leitura ao estado do drone
		state.ProcessFireReading(cell, meta)

		response := map[string]interface{}{
			"message": "Leitura adicionada com sucesso",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// createDeltaHandler cria handler para POST /delta
// Este handler recebe deltas de outros drones e integra no CRDT local
func createDeltaHandler(sensorAPI *sensor.FireSensor, dissemination *gossip.DisseminationSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		// Tenta decodificar como DeltaMsg da disseminação (Fase 4)
		var deltaMsg gossip.DeltaMsg
		if err := json.NewDecoder(r.Body).Decode(&deltaMsg); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// Processa delta recebido via gossip
		if dissemination.IsRunning() {
			if err := dissemination.ProcessReceivedDelta(deltaMsg); err != nil {
				log.Printf("[MAIN] Erro ao processar delta recebido: %v", err)
			}
		}

		// Integra no CRDT local
		state.MergeDelta(deltaMsg.Data)

		response := map[string]interface{}{
			"status":    "received",
			"delta_id":  deltaMsg.ID,
			"ttl":       deltaMsg.TTL,
			"sender_id": deltaMsg.SenderID,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// createStateHandler cria handler para GET /state
func createStateHandler(sensorAPI *sensor.FireSensor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		droneState := state.GetActiveFires()
		latest := state.GetLatestReadings()

		response := map[string]interface{}{
			"all_deltas":      droneState,
			"latest_readings": latest,
			"total_deltas":    len(droneState),
			"unique_sensors":  len(latest),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// createStatsHandler cria handler para GET /stats
func createStatsHandler(sensorAPI *sensor.FireSensor, neighborTable *network.NeighborTable, controlSystem *protocol.ControlSystem, dissemination *gossip.DisseminationSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		sensorStats := sensorAPI.GetStats()
		neighborStats := neighborTable.GetStats()
		controlStats := controlSystem.GetStats()
		disseminationStats := dissemination.GetStats()

		response := map[string]interface{}{
			"sensor_system": sensorStats,
			"network":       neighborStats,
			"control":       controlStats,
			"dissemination": disseminationStats,
			"uptime":        time.Since(startTime).Seconds(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
