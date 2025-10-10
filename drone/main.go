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

var startTime = time.Now() // For uptime calculation

func main() {
	// Command line flags
	var (
		droneID        = flag.String("id", "drone-1", "Unique ID of this drone")
		sampleSec      = flag.Int("sample-sec", 10, "Sensor sampling interval in seconds")
		fanout         = flag.Int("fanout", 3, "Number of neighbors for gossip")
		ttl            = flag.Int("ttl", 4, "Initial TTL for gossip messages")
		deltaPushSec   = flag.Int("delta-push-sec", 5, "Delta push interval in seconds")
		antiEntropySec = flag.Int("anti-entropy-sec", 60, "Anti-entropy interval in seconds")
		udpPort        = flag.Int("udp-port", 7000, "UDP port for control")
		tcpPort        = flag.Int("tcp-port", 8080, "TCP port for data")
		bindAddr       = flag.String("bind", "0.0.0.0", "Bind address")
		showUsage      = flag.Bool("help", false, "Show usage help")
	)
	flag.Parse()

	if *showUsage {
		printUsage()
		return
	}

	// Create drone configuration
	cfg := config.DefaultConfig()
	cfg.DroneID = *droneID
	cfg.SampleInterval = time.Duration(*sampleSec) * time.Second
	cfg.Fanout = *fanout
	cfg.TTL = *ttl
	cfg.DeltaPushInterval = time.Duration(*deltaPushSec) * time.Second
	cfg.AntiEntropyInterval = time.Duration(*antiEntropySec) * time.Second
	cfg.UDPPort = *udpPort
	cfg.TCPPort = *tcpPort
	cfg.BindAddr = *bindAddr

	// Neighbor table
	neighborTable := network.NewNeighborTable(cfg.NeighborTimeout)

	state.InitGlobalState(cfg.DroneID)

	sensorAPI := sensor.NewFireSensor(cfg.DroneID, cfg.SampleInterval, cfg.GridSize.X, cfg.GridSize.Y)

	udpServer := network.NewUDPServer(cfg.DroneID, cfg.UDPPort, neighborTable)
	tcpServer := network.NewTCPServer(cfg.DroneID, cfg.TCPPort)

	// Control system
	controlSystem := protocol.NewControlSystem(cfg.DroneID, sensorAPI, udpServer)

	// Dissemination system with TTL gossip
	tcpSender := gossip.NewHTTPTCPSender(5 * time.Second)
	disseminationSystem := gossip.NewDisseminationSystem(
		cfg.DroneID,
		cfg.Fanout,
		cfg.TTL,
		cfg.DeltaPushInterval,
		cfg.AntiEntropyInterval,
		neighborTable,
		tcpSender,
	)

	// Handlers integration
	tcpServer.SensorHandler = createSensorHandler(sensorAPI, disseminationSystem)
	tcpServer.DeltaHandler = createDeltaHandler(sensorAPI, disseminationSystem)
	tcpServer.StateHandler = createStateHandler(sensorAPI)
	tcpServer.StatsHandler = createStatsHandler(sensorAPI, neighborTable, controlSystem, disseminationSystem)
	tcpServer.PositionHandler = createPositionHandler(sensorAPI)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutdown signal received, stopping...")

		fmt.Println("Stopping control system...")
		controlSystem.Stop()

		fmt.Println("Stopping dissemination system...")
		disseminationSystem.Stop()

		fmt.Println("Stopping sensor collection...")
		sensorAPI.Stop()

		fmt.Println("Stopping UDP server...")
		if err := udpServer.Stop(); err != nil {
			fmt.Printf("Error stopping UDP: %v\n", err)
		}

		fmt.Println("Stopping TCP server...")
		if err := tcpServer.Stop(); err != nil {
			fmt.Printf("Error stopping TCP: %v\n", err)
		}

		os.Exit(0)
	}()

	// Startup info
	fmt.Printf("=== Drone %s ===\n", cfg.DroneID)
	fmt.Printf("UDP (control): %s:%d\n", cfg.BindAddr, cfg.UDPPort)
	fmt.Printf("TCP (data): http://%s:%d\n", cfg.BindAddr, cfg.TCPPort)
	fmt.Printf("Sampling: every %v\n", cfg.SampleInterval)
	fmt.Printf("Gossip: fanout=%d, ttl=%d\n", cfg.Fanout, cfg.TTL)
	fmt.Printf("Dissemination: delta-push=%v, anti-entropy=%v\n", cfg.DeltaPushInterval, cfg.AntiEntropyInterval)
	fmt.Printf("Starting...\n\n")

	// Start components
	sensorAPI.Start()
	controlSystem.Start()
	disseminationSystem.Start()

	if err := udpServer.Start(); err != nil {
		log.Fatalf("Error starting UDP server: %v", err)
	}

	if err := tcpServer.Start(); err != nil {
		log.Fatalf("Error starting TCP server: %v", err)
	}
}

// printUsage shows available options and endpoints
func printUsage() {
	fmt.Fprintf(os.Stderr, `
=== Drone Sensor System ===

USAGE:
  %s [options]

EXAMPLES:
  %s -id=drone-1 -sample-sec=10
  %s -id=drone-2 -sample-sec=5 -fanout=2 -ttl=3
  %s -id=drone-3 -udp-port=7001 -tcp-port=8081
  %s -id=drone-4 -delta-push-sec=3 -anti-entropy-sec=30

OPTIONS:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])

	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, `
ENDPOINTS (TCP):
  POST /sensor     - Add a sensor reading
  POST /delta      - Receive deltas from other drones
  POST /position   - Update drone position {x: int, y: int}
  GET  /state      - Current CRDT state
  GET  /stats      - Drone statistics
`)
}

// createSensorHandler handles POST /sensor
func createSensorHandler(sensorAPI *sensor.FireSensor, dissemination *gossip.DisseminationSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var reading sensor.FireReading
		if err := json.NewDecoder(r.Body).Decode(&reading); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		cell := crdt.Cell{X: reading.X, Y: reading.Y}
		meta := crdt.FireMeta{Timestamp: reading.Timestamp, Confidence: reading.Confidence}

		state.AddFire(cell, meta)

		response := map[string]interface{}{
			"message": "Reading successfully added",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// createDeltaHandler handles POST /delta
func createDeltaHandler(sensorAPI *sensor.FireSensor, dissemination *gossip.DisseminationSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var deltaMsg gossip.DeltaMsg
		if err := json.NewDecoder(r.Body).Decode(&deltaMsg); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if dissemination.IsRunning() {
			if err := dissemination.ProcessReceivedDelta(deltaMsg); err != nil {
				log.Printf("[MAIN] Error processing received delta: %v", err)
			}
		}

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

// createStateHandler handles GET /state
func createStateHandler(sensorAPI *sensor.FireSensor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

// createStatsHandler handles GET /stats
func createStatsHandler(sensorAPI *sensor.FireSensor, neighborTable *network.NeighborTable, controlSystem *protocol.ControlSystem, dissemination *gossip.DisseminationSystem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

func createPositionHandler(sensorAPI *sensor.FireSensor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var position struct {
			X int `json:"x"`
			Y int `json:"y"`
		}

		if err := json.NewDecoder(r.Body).Decode(&position); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if err := sensorAPI.SetPosition(position.X, position.Y); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"message": "Position updated successfully",
			"x":       position.X,
			"y":       position.Y,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
