package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/heitortanoue/tcc/sensor"
	"github.com/heitortanoue/tcc/swim"
)

// PeerClient representa um cliente para comunicação com peers usando SWIM
type PeerClient struct {
	membership *swim.MembershipManager // gerenciador de membership SWIM
	crdt       *sensor.SensorCRDT      // referência ao CRDT local
	droneID    string                  // ID deste drone
	httpClient *http.Client            // cliente HTTP reutilizável
}

// NewPeerClient cria um novo cliente para gossip usando SWIM
func NewPeerClient(droneID string, crdt *sensor.SensorCRDT, membership *swim.MembershipManager) *PeerClient {
	return &PeerClient{
		droneID:    droneID,
		crdt:       crdt,
		membership: membership,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // timeout para requisições HTTP
		},
	}
}

// StartGossip inicia o processo de gossip anti-entropy
func (p *PeerClient) StartGossip(intervalSeconds int) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	go func() {
		for range ticker.C {
			fmt.Printf("[GOSSIP] Iniciando gossip para %s\n", p.droneID)
			p.gossipToPeers()
		}
	}()
}

// gossipToPeers envia deltas pendentes para todos os peers ativos descobertos via SWIM
func (p *PeerClient) gossipToPeers() {
	pending := p.crdt.GetPendingDeltas()

	// Se não há deltas pendentes, não faz nada
	if len(pending) == 0 {
		return
	}

	// Cria o lote de deltas
	batch := sensor.DeltaBatch{
		SenderID: p.droneID,
		Deltas:   pending,
	}

	// Obtém URLs dos peers ativos via SWIM memberlist
	peerURLs := p.membership.GetMemberURLs()
	if len(peerURLs) == 0 {
		fmt.Printf("[GOSSIP] Nenhum peer ativo encontrado via SWIM\n")
		return
	}

	// Envia para todos os peers descobertos
	successCount := 0
	for _, peerURL := range peerURLs {
		if p.sendDeltaToPeer(peerURL, batch) {
			fmt.Printf("[GOSSIP] Enviado %d deltas para %s\n", len(pending), peerURL)
			successCount++
		}
	}

	// Se conseguiu enviar para pelo menos um peer, limpa o buffer
	if successCount > 0 {
		p.crdt.ClearPendingDeltas()
		fmt.Printf("[GOSSIP] Enviados %d deltas para %d/%d peers (via SWIM)\n",
			len(pending), successCount, len(peerURLs))
	}
}

// sendDeltaToPeer envia um lote de deltas para um peer específico com retry
func (p *PeerClient) sendDeltaToPeer(peerURL string, batch sensor.DeltaBatch) bool {
	// Serializa o lote
	jsonData, err := json.Marshal(batch)
	if err != nil {
		fmt.Printf("[GOSSIP] Erro ao serializar lote: %v\n", err)
		return false
	}

	// Tenta enviar com retry simples
	maxRetries := 2
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff exponencial: 1s, 2s
			backoff := time.Duration(1<<attempt) * time.Second
			time.Sleep(backoff)
			fmt.Printf("[GOSSIP] Retry %d/%d para %s\n", attempt, maxRetries, peerURL)
		}

		// Envia POST para o peer
		url := peerURL + "/delta"
		resp, err := p.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			if attempt == maxRetries {
				fmt.Printf("[GOSSIP] Falha final ao enviar para %s: %v\n", url, err)
			}
			continue
		}

		resp.Body.Close() // fecha imediatamente já que não lemos o body

		if resp.StatusCode == http.StatusOK {
			return true
		}

		if attempt == maxRetries {
			fmt.Printf("[GOSSIP] Resposta não-OK de %s: %d\n", url, resp.StatusCode)
		}
	}

	return false
}

// PullFromPeer solicita deltas de um peer específico (anti-entropy pull)
func (p *PeerClient) PullFromPeer(peerURL string) error {
	// Busca deltas do peer
	resp, err := p.httpClient.Get(peerURL + "/deltas")
	if err != nil {
		return fmt.Errorf("erro ao buscar deltas de %s: %v", peerURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resposta não-OK de %s: %d", peerURL, resp.StatusCode)
	}

	// Decodifica a resposta
	var deltasResp struct {
		Pending []sensor.SensorDelta `json:"pending"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&deltasResp); err != nil {
		return fmt.Errorf("erro ao decodificar resposta de %s: %v", peerURL, err)
	}

	// Se há deltas, faz merge
	if len(deltasResp.Pending) > 0 {
		batch := sensor.DeltaBatch{
			SenderID: "pull-from-" + peerURL,
			Deltas:   deltasResp.Pending,
		}
		mergedCount := p.crdt.Merge(batch)
		fmt.Printf("[PULL] Merged %d deltas de %s\n", mergedCount, peerURL)
	}

	return nil
}

// PullFromAllPeers executa pull de todos os peers ativos
func (p *PeerClient) PullFromAllPeers() error {
	peerURLs := p.membership.GetMemberURLs()
	if len(peerURLs) == 0 {
		return fmt.Errorf("nenhum peer ativo encontrado")
	}

	successCount := 0
	for _, peerURL := range peerURLs {
		if err := p.PullFromPeer(peerURL); err != nil {
			fmt.Printf("[PULL] Erro ao puxar de %s: %v\n", peerURL, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("[PULL] Pull bem-sucedido de %d/%d peers\n", successCount, len(peerURLs))
	return nil
}

// GetPeerURLs retorna a lista de URLs dos peers ativos via SWIM
func (p *PeerClient) GetPeerURLs() []string {
	return p.membership.GetMemberURLs()
}

// GetActivePeerCount retorna o número de peers ativos
func (p *PeerClient) GetActivePeerCount() int {
	return len(p.membership.GetLiveMembers())
}

// GetMembershipStats retorna estatísticas do membership
func (p *PeerClient) GetMembershipStats() map[string]interface{} {
	return p.membership.GetStats()
}
