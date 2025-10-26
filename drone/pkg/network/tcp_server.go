package network

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

type TCPServer struct {
	port    int
	mux     *http.ServeMux
	droneID string
	server  *http.Server

	SensorHandler   http.HandlerFunc
	DeltaHandler    http.HandlerFunc
	StateHandler    http.HandlerFunc
	StatsHandler    http.HandlerFunc
	CleanupHandler  http.HandlerFunc
	PositionHandler http.HandlerFunc
}

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

// setupRoutes configures basic HTTP routes
func (s *TCPServer) setupRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/sensor", s.handleSensorWrapper)
	s.mux.HandleFunc("/delta", s.handleDeltaWrapper)
	s.mux.HandleFunc("/state", s.handleStateWrapper)
	s.mux.HandleFunc("/stats", s.handleStatsWrapper)
	s.mux.HandleFunc("/cleanup", s.handleCleanupWrapper)
	s.mux.HandleFunc("/position", s.handlePositionWrapper)
}

// Start launches the TCP server
func (s *TCPServer) Start() error {
	log.Printf("[TCP] Server started on port %d", s.port)
	return s.server.ListenAndServe()
}

// Stop shuts down the TCP server
func (s *TCPServer) Stop() error {
	log.Printf("[TCP] Stopping server on port %d", s.port)
	return s.server.Close()
}

// handleHealth provides a basic health-check endpoint
func (s *TCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"drone_id": s.droneID,
		"status":   "healthy",
		"port":     s.port,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Wrappers for external handlers (to be implemented in later phases)
func (s *TCPServer) handleSensorWrapper(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Message-Type", "SENSOR")
	w.Header().Set("X-Drone-ID", s.droneID)
	if s.SensorHandler != nil {
		s.SensorHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Sensor handler")
	}
}

func (s *TCPServer) handleDeltaWrapper(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Message-Type", "DELTA")
	w.Header().Set("X-Drone-ID", s.droneID)
	if s.DeltaHandler != nil {
		s.DeltaHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Delta handler")
	}
}

func (s *TCPServer) handleStateWrapper(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Message-Type", "STATE")
	w.Header().Set("X-Drone-ID", s.droneID)
	if s.StateHandler != nil {
		s.StateHandler(w, r)
	} else {
		s.sendNotImplemented(w, "State handler")
	}
}

func (s *TCPServer) handleStatsWrapper(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Message-Type", "STATS")
	w.Header().Set("X-Drone-ID", s.droneID)
	if s.StatsHandler != nil {
		s.StatsHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Stats handler")
	}
}

func (s *TCPServer) handleCleanupWrapper(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Message-Type", "CLEANUP")
	w.Header().Set("X-Drone-ID", s.droneID)
	if s.CleanupHandler != nil {
		s.CleanupHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Cleanup handler")
	}
}

func (s *TCPServer) handlePositionWrapper(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Message-Type", "POSITION")
	w.Header().Set("X-Drone-ID", s.droneID)
	if s.PositionHandler != nil {
		s.PositionHandler(w, r)
	} else {
		s.sendNotImplemented(w, "Position handler")
	}
}

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

// GetStats returns TCP server statistics
func (s *TCPServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"tcp_port": s.port,
		"drone_id": s.droneID,
	}
}
