package network

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// TCPServer gerencia comunicação TCP na porta 8080 (canal de dados)
type TCPServer struct {
	port    int
	mux     *http.ServeMux
	droneID string
	server  *http.Server

	// Handlers que podem ser definidos externamente
	SensorHandler  http.HandlerFunc
	DeltaHandler   http.HandlerFunc
	StateHandler   http.HandlerFunc
	StatsHandler   http.HandlerFunc
	CleanupHandler http.HandlerFunc
}

// NewTCPServer cria um novo servidor TCP
func NewTCPServer(droneID string, port int) *TCPServer {
	mux := http.NewServeMux()

	tcpServer := &TCPServer{
		droneID: droneID,
		port:    port,
		mux:     mux,
		server: &http.Server{
			Addr:    ":" + strconv.Itoa(port),
			Handler: mux,
		},
	}

	tcpServer.setupRoutes()
	return tcpServer
}

// setupRoutes configura as rotas HTTP básicas
func (s *TCPServer) setupRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/sensor", s.handleSensorWrapper)
	s.mux.HandleFunc("/delta", s.handleDeltaWrapper)
	s.mux.HandleFunc("/state", s.handleStateWrapper)
	s.mux.HandleFunc("/stats", s.handleStatsWrapper)
}

// Start inicia o servidor TCP
func (s *TCPServer) Start() error {
	log.Printf("[TCP] Servidor iniciado na porta %d", s.port)
	return s.server.ListenAndServe()
}

// Stop para o servidor TCP
func (s *TCPServer) Stop() error {
	log.Printf("[TCP] Parando servidor na porta %d", s.port)
	return s.server.Close()
}

// handleHealth endpoint básico de saúde
func (s *TCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"drone_id": s.droneID,
		"status":   "healthy",
		"port":     s.port,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Wrappers para handlers externos (implementados nas próximas fases)
func (s *TCPServer) handleSensorWrapper(w http.ResponseWriter, r *http.Request) {
	if s.SensorHandler != nil {
		s.SensorHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Sensor handler")
	}
}

func (s *TCPServer) handleDeltaWrapper(w http.ResponseWriter, r *http.Request) {
	if s.DeltaHandler != nil {
		s.DeltaHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Delta handler")
	}
}

func (s *TCPServer) handleStateWrapper(w http.ResponseWriter, r *http.Request) {
	if s.StateHandler != nil {
		s.StateHandler(w, r)
	} else {
		s.sendNotImplemented(w, "State handler")
	}
}

func (s *TCPServer) handleStatsWrapper(w http.ResponseWriter, r *http.Request) {
	if s.StatsHandler != nil {
		s.StatsHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Stats handler")
	}
}

// sendNotImplemented envia resposta de funcionalidade não implementada
func (s *TCPServer) sendNotImplemented(w http.ResponseWriter, feature string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)

	response := map[string]interface{}{
		"error":   "Not implemented",
		"feature": feature,
		"phase":   "Will be implemented in next phases",
	}

	json.NewEncoder(w).Encode(response)
}

// GetStats retorna estatísticas do servidor TCP
func (s *TCPServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"tcp_port": s.port,
		"drone_id": s.droneID,
	}
}
