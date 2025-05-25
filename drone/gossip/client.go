package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/heitortanoue/tcc/sensor"
)

// PeerClient representa um cliente para comunicação com peers
type PeerClient struct {
	peerURLs []string           // URLs dos peers
	crdt     *sensor.SensorCRDT // referência ao CRDT local
	droneID  string             // ID deste drone
}

// NewPeerClient cria um novo cliente para gossip
func NewPeerClient(droneID string, crdt *sensor.SensorCRDT, peerURLs []string) *PeerClient {
	return &PeerClient{
		droneID:  droneID,
		crdt:     crdt,
		peerURLs: peerURLs,
	}
}

// StartGossip inicia o processo de gossip anti-entropy
func (p *PeerClient) StartGossip(intervalSeconds int) {
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	go func() {
		for range ticker.C {
			p.gossipToPeers()
		}
	}()
}

// gossipToPeers envia deltas pendentes para todos os peers
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

	// Envia para todos os peers
	successCount := 0
	for _, peerURL := range p.peerURLs {
		if p.sendDeltaToPeer(peerURL, batch) {
			successCount++
		}
	}

	// Se conseguiu enviar para pelo menos um peer, limpa o buffer
	if successCount > 0 {
		p.crdt.ClearPendingDeltas()
		fmt.Printf("[GOSSIP] Enviados %d deltas para %d/%d peers\n",
			len(pending), successCount, len(p.peerURLs))
	}
}

// sendDeltaToPeer envia um lote de deltas para um peer específico
func (p *PeerClient) sendDeltaToPeer(peerURL string, batch sensor.DeltaBatch) bool {
	// Serializa o lote
	jsonData, err := json.Marshal(batch)
	if err != nil {
		fmt.Printf("[GOSSIP] Erro ao serializar lote: %v\n", err)
		return false
	}

	// Envia POST para o peer
	url := peerURL + "/delta"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("[GOSSIP] Erro ao enviar para %s: %v\n", url, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[GOSSIP] Resposta não-OK de %s: %d\n", url, resp.StatusCode)
		return false
	}

	return true
}

// PullFromPeer solicita deltas de um peer específico (anti-entropy pull)
func (p *PeerClient) PullFromPeer(peerURL string) error {
	// Busca deltas do peer
	resp, err := http.Get(peerURL + "/deltas")
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

// GetPeerURLs retorna a lista de URLs dos peers
func (p *PeerClient) GetPeerURLs() []string {
	return p.peerURLs
}

// AddPeer adiciona um novo peer à lista
func (p *PeerClient) AddPeer(peerURL string) {
	p.peerURLs = append(p.peerURLs, peerURL)
}
