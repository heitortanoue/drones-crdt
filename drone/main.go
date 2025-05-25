package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/heitortanoue/tcc/api"
)

func main() {
	// Flags da linha de comando
	var (
		droneID   = flag.String("drone", "drone-01", "ID único deste drone")
		apiPort   = flag.Int("port", 8080, "Porta para o servidor HTTP da API")
		swimPort  = flag.Int("swim-port", 7946, "Porta para o protocolo SWIM")
		bindAddr  = flag.String("bind", "0.0.0.0", "Endereço para bind do SWIM")
		seedNodes = flag.String("seeds", "", "Nós seeds separados por vírgula (ex: drone-02,drone-03)")
		showUsage = flag.Bool("help", false, "Mostra ajuda de uso")
	)
	flag.Parse()

	if *showUsage {
		printUsage()
		return
	}

	// Processa a lista de seeds
	var seeds []string
	if *seedNodes != "" {
		seeds = strings.Split(*seedNodes, ",")
		for i, seed := range seeds {
			seeds[i] = strings.TrimSpace(seed)
		}
	}

	// Configuração do drone
	config := api.DroneConfig{
		DroneID:   *droneID,
		APIPort:   *apiPort,
		SWIMPort:  *swimPort,
		BindAddr:  *bindAddr,
		SeedNodes: seeds,
	}

	// Cria o servidor do drone
	server, err := api.NewDroneServer(config)
	if err != nil {
		log.Fatalf("Erro ao criar servidor: %v", err)
	}

	// Setup graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nRecebido sinal de interrupção, desligando...")
		if err := server.Shutdown(); err != nil {
			fmt.Printf("Erro ao desligar: %v\n", err)
		}
		os.Exit(0)
	}()

	// Mostra informações de inicialização
	fmt.Printf("=== Drone %s ===\n", *droneID)
	fmt.Printf("API REST: http://%s:%d\n", *bindAddr, *apiPort)
	fmt.Printf("SWIM: %s:%d\n", *bindAddr, *swimPort)

	if len(seeds) > 0 {
		fmt.Printf("Seeds: %v\n", seeds)
	} else {
		fmt.Printf("Modo standalone (primeiro nó do cluster)\n")
	}

	fmt.Printf("Iniciando...\n\n")

	// Inicia o servidor (bloqueia até terminar)
	err = server.Start()
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}

// printUsage mostra exemplos de uso
func printUsage() {
	fmt.Fprintf(os.Stderr, `
=== Drone SWIM Cluster ===

USAGE:
  %s [opções]

EXAMPLES:
  # Primeiro drone do cluster (seed)
  %s -drone=drone-01 -port=8080

  # Segundo drone conectando ao primeiro
  %s -drone=drone-02 -port=8081 -seeds=drone-01

  # Drone com porta SWIM customizada
  %s -drone=drone-03 -port=8082 -swim-port=7947 -seeds=drone-01,drone-02

  # Cluster com bind específico (útil em Docker)
  %s -drone=drone-01 -bind=0.0.0.0 -port=8080 -swim-port=7946

OPTIONS:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])

	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, `
ENDPOINTS:
  GET  /stats     - Estatísticas do drone e cluster
  GET  /members   - Lista membros do cluster SWIM
  GET  /state     - Estado atual do CRDT
  GET  /deltas    - Deltas pendentes para gossip
  POST /sensor    - Adiciona leitura de sensor
  POST /delta     - Recebe deltas de outros drones
  POST /join      - Conecta a um nó específico
  POST /cleanup   - Limpa deltas antigos

NOTES:
  - Porta SWIM (padrão 7946) usada para membership/failure detection
  - Porta API (padrão 8080) usada para REST API e gossip δ-CRDT
  - Seeds são IDs de nós, não URLs (ex: "drone-01", não "http://drone-01:8080")
  - Failure detection automática em ~5s via protocolo SWIM
  - Anti-entropy gossip a cada 30s entre membros descobertos

`)
}
