package config

import (
	"time"
)

// DroneConfig configuração centralizada do drone
type DroneConfig struct {
	// Identificação
	DroneID string `json:"drone_id"`

	// Rede
	UDPPort  int    `json:"udp_port"`  // porta 7000 para controle
	TCPPort  int    `json:"tcp_port"`  // porta 8080 para dados
	BindAddr string `json:"bind_addr"` // endereço para bind

	// Coleta de dados
	SampleInterval time.Duration `json:"sample_interval"` // intervalo entre leituras

	// Gossip
	Fanout int `json:"fanout"` // número de vizinhos para gossip
	TTL    int `json:"ttl"`    // TTL inicial para mensagens

	// Dissemination intervals
	DeltaPushInterval   time.Duration `json:"delta_push_interval"`   // intervalo entre envios de delta (5s)
	AntiEntropyInterval time.Duration `json:"anti_entropy_interval"` // intervalo de anti-entropy (60s)

	// Timeouts e intervalos
	NeighborTimeout    time.Duration `json:"neighbor_timeout"`    // timeout para expirar vizinhos (9s)
	TransmitterTimeout time.Duration `json:"transmitter_timeout"` // timeout para transmissor (5s)
}

// DefaultConfig retorna configuração padrão
func DefaultConfig() *DroneConfig {
	return &DroneConfig{
		DroneID:             "drone-1",
		UDPPort:             7000,
		TCPPort:             8080,
		BindAddr:            "0.0.0.0",
		SampleInterval:      10 * time.Second,
		Fanout:              3,
		TTL:                 4,
		DeltaPushInterval:   5 * time.Second,  // Delta push a cada 5s
		AntiEntropyInterval: 60 * time.Second, // Anti-entropy a cada 60s
		NeighborTimeout:     9 * time.Second,
		TransmitterTimeout:  5 * time.Second,
	}
}
