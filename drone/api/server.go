package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/heitortanoue/tcc/gossip"
	"github.com/heitortanoue/tcc/logging"
	"github.com/heitortanoue/tcc/sensor"
)

// DroneServer representa o servidor HTTP do drone
type DroneServer struct {
	crdt           *sensor.SensorCRDT
	port           int
	mux            *http.ServeMux
	logger         *logging.DroneLogger
	nodeManager    *gossip.NodeManager
}

// NewDroneServer cria uma nova instância do servidor
func NewDroneServer(droneID string, port int) *DroneServer {
	crdt := sensor.NewSensorCRDT(droneID)
	server := &DroneServer{
		crdt:           crdt,
		port:           port,
		mux:            http.NewServeMux(),
		logger:         logging.NewDroneLogger(droneID),
		nodeManager:    gossip.NewNodeManager(droneID, crdt, []string{}),
	}
	server.setupRoutes()
	return server
}

// NewDroneServerWithPeers cria uma nova instância do servidor com peers iniciais
func NewDroneServerWithPeers(droneID string, port int, initialPeers []string) *DroneServer {
	crdt := sensor.NewSensorCRDT(droneID)
	server := &DroneServer{
		crdt:           crdt,
		port:           port,
		mux:            http.NewServeMux(),
		logger:         logging.NewDroneLogger(droneID),
		nodeManager:    gossip.NewNodeManager(droneID, crdt, initialPeers),
	}
	server.setupRoutes()
	return server
}

// setupRoutes configura as rotas da API
func (s *DroneServer) setupRoutes() {
	s.mux.HandleFunc("/sensor", s.handleSensor)
	s.mux.HandleFunc("/deltas", s.handleGetDeltas)
	s.mux.HandleFunc("/delta", s.handlePostDelta)
	s.mux.HandleFunc("/state", s.handleGetState)
	s.mux.HandleFunc("/handshake", s.handleHandshake)
	s.mux.HandleFunc("/peers", s.handleGetPeers)
	s.mux.HandleFunc("/cleanup", s.handleCleanup)
	s.mux.HandleFunc("/stats", s.handleStats)
}

// Start inicia o servidor HTTP
func (s *DroneServer) Start() error {
	fmt.Printf("Iniciando servidor do drone na porta %d\n", s.port)
	return http.ListenAndServe(":"+strconv.Itoa(s.port), s.mux)
}

// Estruturas para respostas da API
type SensorResponse struct {
	Delta sensor.SensorDelta `json:"delta"`
}

type DeltasResponse struct {
	Pending []sensor.SensorDelta `json:"pending"`
}

type MergeResponse struct {
	MergedCount  int `json:"merged_count"`
	CurrentTotal int `json:"current_total"`
}

type StateResponse struct {
	State []sensor.SensorDelta `json:"state"`
}

type PeersResponse struct {
	Peers []string `json:"peers"`
}

type CleanupRequest struct {
	MaxAgeHours *int64 `json:"max_age_hours,omitempty"`
	MaxSize     *int   `json:"max_size,omitempty"`
}

type CleanupResponse struct {
	RemovedDeltas   int               `json:"removed_deltas"`
	RemainingDeltas int               `json:"remaining_deltas"`
	MemoryStats     map[string]uint64 `json:"memory_stats"`
}

type StatsResponse struct {
	MemoryStats    map[string]uint64 `json:"memory_stats"`
	LatestBySensor int               `json:"latest_by_sensor"`
	ServerUptime   string            `json:"server_uptime"`
	NodePeers      int               `json:"node_peers"`
	ActiveSensors  int               `json:"active_sensors"`
}

type SensorsResponse struct {
	Sensors map[string]interface{} `json:"sensors"`
}

// handleSensor processa POST /sensor
func (s *DroneServer) handleSensor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var reading sensor.SensorReading
	if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Gera timestamp se não fornecido
	if reading.Timestamp == 0 {
		reading.Timestamp = sensor.GenerateTimestamp()
	}

	// Adiciona a leitura ao CRDT
	delta := s.crdt.AddDelta(reading)

	// Log da leitura recebida
	s.logger.LogSensorReading(delta)

	// Retorna o delta gerado
	response := SensorResponse{Delta: *delta}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleGetDeltas processa GET /deltas
func (s *DroneServer) handleGetDeltas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	pending := s.crdt.GetPendingDeltas()
	response := DeltasResponse{Pending: pending}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePostDelta processa POST /delta
func (s *DroneServer) handlePostDelta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var batch sensor.DeltaBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Faz merge dos deltas recebidos
	mergedCount := s.crdt.Merge(batch)
	totalCount := s.crdt.GetTotalDeltasCount()

	// Log do merge de deltas
	s.logger.LogDeltaReceived(batch.SenderID, batch.Deltas, mergedCount)

	response := MergeResponse{
		MergedCount:  mergedCount,
		CurrentTotal: totalCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetState processa GET /state
func (s *DroneServer) handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	state := s.crdt.GetState()
	response := StateResponse{State: state}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHandshake processa POST /handshake
func (s *DroneServer) handleHandshake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var handshakeReq gossip.HandshakeRequest
	if err := json.NewDecoder(r.Body).Decode(&handshakeReq); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Processa o handshake usando o NodeManager
	response := s.nodeManager.HandleJoinRequest(&handshakeReq)

	// Log do handshake
	s.logger.LogGossipEvent(fmt.Sprintf("Handshake de %s: %v", handshakeReq.DroneID, response.Success))

	w.Header().Set("Content-Type", "application/json")
	if response.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}

// handleGetPeers processa GET /peers
func (s *DroneServer) handleGetPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	peers := s.nodeManager.GetPeers()
	response := PeersResponse{Peers: peers}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCleanup processa POST /cleanup
func (s *DroneServer) handleCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var cleanupReq CleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&cleanupReq); err != nil {
		// Se não há body JSON, usa valores padrão
		cleanupReq = CleanupRequest{}
	}

	removedCount := 0

	// Limpeza por idade se especificado
	if cleanupReq.MaxAgeHours != nil {
		maxAge := time.Duration(*cleanupReq.MaxAgeHours) * time.Hour
		removed := s.crdt.CleanupOldDeltasByAge(maxAge)
		removedCount += removed
		s.logger.LogGossipEvent(fmt.Sprintf("Limpeza por idade: %d deltas removidos", removed))
	}

	// Limpeza por tamanho se especificado
	if cleanupReq.MaxSize != nil {
		removed := s.crdt.TrimToMaxSize(*cleanupReq.MaxSize)
		removedCount += removed
		s.logger.LogGossipEvent(fmt.Sprintf("Limpeza por tamanho: %d deltas removidos", removed))
	}

	// Se nenhum parâmetro foi especificado, faz limpeza padrão
	if cleanupReq.MaxAgeHours == nil && cleanupReq.MaxSize == nil {
		// Remove deltas mais antigos que 1 hora
		maxAge := time.Hour
		removedCount = s.crdt.CleanupOldDeltasByAge(maxAge)
		s.logger.LogGossipEvent(fmt.Sprintf("Limpeza padrão: %d deltas removidos", removedCount))
	}

	response := CleanupResponse{
		RemovedDeltas:   removedCount,
		RemainingDeltas: s.crdt.GetTotalDeltasCount(),
		MemoryStats:     s.crdt.GetMemoryStats(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStats processa GET /stats
func (s *DroneServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(time.Now().Add(-time.Hour)).Round(time.Second).String() // placeholder

	response := StatsResponse{
		MemoryStats:    s.crdt.GetMemoryStats(),
		LatestBySensor: len(s.crdt.GetState()),
		ServerUptime:   uptime,
		NodePeers:      len(s.nodeManager.GetPeers()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// JoinNetwork solicita entrada na rede
func (s *DroneServer) JoinNetwork(targetPeerURL string) error {
	response, err := s.nodeManager.RequestJoin(targetPeerURL)
	if err != nil {
		return fmt.Errorf("erro ao fazer handshake: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("handshake rejeitado: %s", response.Message)
	}

	s.logger.LogGossipEvent(fmt.Sprintf("Conectado à rede via %s", targetPeerURL))
	return nil
}

// GetCRDT retorna a instância do CRDT (para testes e funcionalidades avançadas)
func (s *DroneServer) GetCRDT() *sensor.SensorCRDT {
	return s.crdt
}

// GetNodeManager retorna a instância do NodeManager
func (s *DroneServer) GetNodeManager() *gossip.NodeManager {
	return s.nodeManager
}
