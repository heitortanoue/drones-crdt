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

	SampleInterval time.Duration `json:"sample_interval"`

	Fanout int `json:"fanout"`
	TTL    int `json:"ttl"`

	DeltaPushInterval   time.Duration `json:"delta_push_interval"`
	AntiEntropyInterval time.Duration `json:"anti_entropy_interval"`

	NeighborTimeout    time.Duration `json:"neighbor_timeout"`
	TransmitterTimeout time.Duration `json:"transmitter_timeout"`

	GridSize GridSize `json:"grid_size"`
}

func DefaultConfig() *DroneConfig {
	return &DroneConfig{
		DroneID:             "drone-1",
		UDPPort:             7000,
		TCPPort:             8080,
		BindAddr:            "0.0.0.0",
		SampleInterval:      10 * time.Second,
		Fanout:              3,
		TTL:                 4,
		DeltaPushInterval:   5 * time.Second,
		AntiEntropyInterval: 60 * time.Second,
		NeighborTimeout:     9 * time.Second,
		TransmitterTimeout:  5 * time.Second,
		GridSize:            GridSize{X: 1000, Y: 1000},
	}
}
