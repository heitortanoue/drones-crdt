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
	"github.com/heitortanoue/tcc/swim"
)

// DroneServer representa o servidor HTTP do drone usando SWIM membership
type DroneServer struct {
	crdt       *sensor.SensorCRDT
	port       int
	mux        *http.ServeMux
	logger     *logging.DroneLogger
	membership *swim.MembershipManager
	peerClient *gossip.PeerClient
	droneID    string
}

// DroneConfig configuração para criar um DroneServer
type DroneConfig struct {
	DroneID   string   // ID único do drone
	APIPort   int      // porta da API REST
	SWIMPort  int      // porta do SWIM (padrão 7946)
	BindAddr  string   // endereço para bind (padrão "0.0.0.0")
	SeedNodes []string // lista de nós seeds para conectar
}

// NewDroneServer cria uma nova instância do servidor usando SWIM
func NewDroneServer(config DroneConfig) (*DroneServer, error) {
	// Cria o CRDT
	crdt := sensor.NewSensorCRDT(config.DroneID)

	// Configuração do membership SWIM
	membershipConfig := swim.MembershipConfig{
		NodeID:   config.DroneID,
		BindAddr: config.BindAddr,
		BindPort: config.SWIMPort,
		APIPort:  config.APIPort,
		Seeds:    config.SeedNodes,
	}

	// Cria o gerenciador de membership
	membership, err := swim.NewMembershipManager(membershipConfig)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar membership: %v", err)
	}

	// Cria o cliente de gossip
	peerClient := gossip.NewPeerClient(config.DroneID, crdt, membership)

	// Cria o servidor
	server := &DroneServer{
		crdt:       crdt,
		port:       config.APIPort,
		mux:        http.NewServeMux(),
		logger:     logging.NewDroneLogger(config.DroneID),
		membership: membership,
		peerClient: peerClient,
		droneID:    config.DroneID,
	}

	server.setupRoutes()
	return server, nil
}

// setupRoutes configura as rotas da API
func (s *DroneServer) setupRoutes() {
	s.mux.HandleFunc("/sensor", s.handleSensor)
	s.mux.HandleFunc("/deltas", s.handleGetDeltas)
	s.mux.HandleFunc("/delta", s.handlePostDelta)
	s.mux.HandleFunc("/state", s.handleGetState)
	s.mux.HandleFunc("/members", s.handleGetMembers)
	s.mux.HandleFunc("/join", s.handleJoinCluster)
	s.mux.HandleFunc("/cleanup", s.handleCleanup)
	s.mux.HandleFunc("/stats", s.handleStats)
}

// Start inicia o servidor HTTP e o gossip
func (s *DroneServer) Start() error {
	// Inicia o gossip anti-entropy (a cada 30 segundos)
	s.peerClient.StartGossip(30)

	fmt.Printf("Iniciando servidor do drone %s na porta %d (SWIM: %s)\n",
		s.droneID, s.port, s.membership.GetLocalAddr())

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

type MembersResponse struct {
	Members []MemberInfo `json:"members"`
	Total   int          `json:"total"`
}

type MemberInfo struct {
	NodeID  string `json:"node_id"`
	Address string `json:"address"`
	Status  string `json:"status"`
}

type JoinRequest struct {
	NodeAddress string `json:"node_address"`
}

type JoinResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
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
	DroneID        string                 `json:"drone_id"`
	MemoryStats    map[string]uint64      `json:"memory_stats"`
	LatestBySensor int                    `json:"latest_by_sensor"`
	ServerUptime   string                 `json:"server_uptime"`
	ActivePeers    int                    `json:"active_peers"`
	Membership     map[string]interface{} `json:"membership"`
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

// handleGetMembers processa GET /members - lista membros do cluster SWIM
func (s *DroneServer) handleGetMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	liveMembers := s.membership.GetLiveMembers()
	members := make([]MemberInfo, 0, len(liveMembers)+1)

	// Adiciona este nó
	members = append(members, MemberInfo{
		NodeID:  s.droneID,
		Address: s.membership.GetLocalAddr(),
		Status:  "local",
	})

	// Adiciona outros membros
	for _, member := range liveMembers {
		members = append(members, MemberInfo{
			NodeID:  member.Name,
			Address: member.Address(),
			Status:  "alive",
		})
	}

	response := MembersResponse{
		Members: members,
		Total:   len(members),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleJoinCluster processa POST /join - conecta a um nó específico
func (s *DroneServer) handleJoinCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var joinReq JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&joinReq); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	// Tenta conectar ao nó especificado
	err := s.membership.JoinNode(joinReq.NodeAddress)

	var response JoinResponse
	if err != nil {
		response = JoinResponse{
			Success: false,
			Message: fmt.Sprintf("Erro ao conectar: %v", err),
		}
		w.WriteHeader(http.StatusBadRequest)
	} else {
		response = JoinResponse{
			Success: true,
			Message: "Conectado com sucesso",
		}
		s.logger.LogGossipEvent(fmt.Sprintf("Conectado ao cluster via %s", joinReq.NodeAddress))
	}

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
		DroneID:        s.droneID,
		MemoryStats:    s.crdt.GetMemoryStats(),
		LatestBySensor: len(s.crdt.GetState()),
		ServerUptime:   uptime,
		ActivePeers:    s.peerClient.GetActivePeerCount(),
		Membership:     s.membership.GetStats(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// JoinNetwork solicita entrada na rede via um nó específico
func (s *DroneServer) JoinNetwork(targetNodeAddr string) error {
	err := s.membership.JoinNode(targetNodeAddr)
	if err != nil {
		return fmt.Errorf("erro ao conectar ao cluster: %v", err)
	}

	s.logger.LogGossipEvent(fmt.Sprintf("Conectado à rede via %s", targetNodeAddr))
	return nil
}

// GetCRDT retorna a instância do CRDT
func (s *DroneServer) GetCRDT() *sensor.SensorCRDT {
	return s.crdt
}

// GetMembership retorna a instância do MembershipManager
func (s *DroneServer) GetMembership() *swim.MembershipManager {
	return s.membership
}

// GetPeerClient retorna a instância do PeerClient
func (s *DroneServer) GetPeerClient() *gossip.PeerClient {
	return s.peerClient
}

// Shutdown desliga o servidor gracefully
func (s *DroneServer) Shutdown() error {
	fmt.Printf("Desligando servidor do drone %s...\n", s.droneID)

	// Deixa o cluster SWIM gracefully
	if err := s.membership.Leave(); err != nil {
		fmt.Printf("Aviso: erro ao deixar cluster: %v\n", err)
	}

	// Desliga o memberlist
	if err := s.membership.Shutdown(); err != nil {
		fmt.Printf("Aviso: erro ao desligar memberlist: %v\n", err)
	}

	return nil
}
