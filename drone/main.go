package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/heitortanoue/tcc/api"
	"github.com/heitortanoue/tcc/gossip"
)

func main() {
	// Flags da linha de comando
	var (
		droneID        = flag.String("drone", "drone-01", "ID único deste drone")
		port           = flag.Int("port", 8080, "Porta para o servidor HTTP")
		peers          = flag.String("peers", "", "URLs dos peers separadas por vírgula (ex: http://drone-02:8080,http://drone-03:8080)")
		gossipInterval = flag.Int("gossip", 5, "Intervalo do gossip em segundos (0 para desabilitar)")
	)
	flag.Parse()

	// Cria o servidor do drone
	server := api.NewDroneServer(*droneID, *port)

	// Configura gossip se peers foram fornecidos
	if *peers != "" && *gossipInterval > 0 {
		peerURLs := strings.Split(*peers, ",")
		for i, url := range peerURLs {
			peerURLs[i] = strings.TrimSpace(url)
		}

		client := gossip.NewPeerClient(*droneID, server.GetCRDT(), peerURLs)
		client.StartGossip(*gossipInterval)

		fmt.Printf("Gossip configurado com %d peers, intervalo %ds\n", len(peerURLs), *gossipInterval)
		for _, peer := range peerURLs {
			fmt.Printf("  - %s\n", peer)
		}
	}

	// Inicia o servidor
	fmt.Printf("Drone %s iniciando na porta %d\n", *droneID, *port)

	var err = server.Start()
	if err != nil {
		log.Fatal("Erro ao iniciar servidor:", err)
	}
}

// printUsage mostra exemplos de uso
func printUsage() {
	fmt.Fprintf(os.Stderr, `
Uso: %s [opções]

Exemplos:
  # Drone único na porta 8080
  %s -drone=drone-01 -port=8080

  # Drone com gossip para 2 peers
  %s -drone=drone-02 -port=8081 -peers="http://localhost:8080,http://localhost:8082" -gossip=5

  # Drone com descoberta de sensores
  %s -drone=drone-01 -port=8080 -discovery=9999

Opções:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	flag.PrintDefaults()
}
