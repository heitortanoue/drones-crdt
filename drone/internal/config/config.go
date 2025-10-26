package config

import (
	"time"
)

type GridSize struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type DroneConfig struct {
	DroneID string `json:"drone_id"`

	UDPPort  int    `json:"udp_port"`
	TCPPort  int    `json:"tcp_port"`
	BindAddr string `json:"bind_addr"`

	SampleInterval      time.Duration `json:"sample_interval"`
	ConfidenceThreshold float64       `json:"confidence_threshold"` // Minimum confidence to accept fire detection

	Fanout int `json:"fanout"`
	TTL    int `json:"ttl"`

	DeltaPushInterval   time.Duration `json:"delta_push_interval"`
	AntiEntropyInterval time.Duration `json:"anti_entropy_interval"`

	NeighborTimeout    time.Duration `json:"neighbor_timeout"`
	TransmitterTimeout time.Duration `json:"transmitter_timeout"`

	HelloInterval time.Duration `json:"hello_interval"` // Base interval for hello messages
	HelloJitter   time.Duration `json:"hello_jitter"`   // Random jitter added to hello interval

	GridSize GridSize `json:"grid_size"`
}

func DefaultConfig() *DroneConfig {
	return &DroneConfig{
		DroneID:             "drone-1",
		UDPPort:             7000,
		TCPPort:             8080,
		BindAddr:            "0.0.0.0",
		SampleInterval:      10000 * time.Millisecond, // 10 seconds
		Fanout:              3,
		TTL:                 4,
		DeltaPushInterval:   1000 * time.Millisecond,  // 1 second
		AntiEntropyInterval: 60000 * time.Millisecond, // 60 seconds
		NeighborTimeout:     3000 * time.Millisecond,  // 3 seconds
		TransmitterTimeout:  2000 * time.Millisecond,  // 2 seconds
		HelloInterval:       1000 * time.Millisecond,  // 1 second base interval
		HelloJitter:         200 * time.Millisecond,   // ±200ms jitter
		ConfidenceThreshold: 50.0,                     // 50% minimum confidence
		GridSize:            GridSize{X: 2500, Y: 2500},
	}
}
