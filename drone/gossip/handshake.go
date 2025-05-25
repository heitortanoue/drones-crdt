package gossip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/heitortanoue/tcc/sensor"
)

// HandshakeRequest representa uma solicitação de handshake
type HandshakeRequest struct {
	DroneID   string `json:"drone_id"`
	JoinedAt  int64  `json:"joined_at"`
	Endpoints struct {
		Sensor string `json:"sensor"`
		Deltas string `json:"deltas"`
		Delta  string `json:"delta"`
		State  string `json:"state"`
	} `json:"endpoints"`
}

// HandshakeResponse representa a resposta do handshake
type HandshakeResponse struct {
	Success      bool                 `json:"success"`
	Message      string               `json:"message"`
	CurrentState []sensor.SensorDelta `json:"current_state,omitempty"`
	PeerList     []string             `json:"peer_list,omitempty"`
}

// NodeManager gerencia entrada e saída de nós
type NodeManager struct {
	crdt     *sensor.SensorCRDT
	peerURLs []string
	droneID  string
}

// NewNodeManager cria um novo gerenciador de nós
func NewNodeManager(droneID string, crdt *sensor.SensorCRDT, initialPeers []string) *NodeManager {
	return &NodeManager{
		droneID:  droneID,
		crdt:     crdt,
		peerURLs: append([]string{}, initialPeers...),
	}
}

// RequestJoin solicita entrada na rede para um novo nó
func (nm *NodeManager) RequestJoin(targetPeerURL string) (*HandshakeResponse, error) {
	handshake := HandshakeRequest{
		DroneID:  nm.droneID,
		JoinedAt: sensor.GenerateTimestamp(),
	}

	// Define endpoints padrão (assumindo porta do droneID)
	handshake.Endpoints.Sensor = fmt.Sprintf("http://%s/sensor", nm.droneID)
	handshake.Endpoints.Deltas = fmt.Sprintf("http://%s/deltas", nm.droneID)
	handshake.Endpoints.Delta = fmt.Sprintf("http://%s/delta", nm.droneID)
	handshake.Endpoints.State = fmt.Sprintf("http://%s/state", nm.droneID)

	// Serializa e envia request
	jsonData, err := json.Marshal(handshake)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar handshake: %v", err)
	}

	resp, err := http.Post(targetPeerURL+"/handshake", "application/json",
		bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("erro ao enviar handshake: %v", err)
	}
	defer resp.Body.Close()

	var response HandshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta: %v", err)
	}

	// Se bem-sucedido, sincroniza estado inicial
	if response.Success && len(response.CurrentState) > 0 {
		batch := sensor.DeltaBatch{
			SenderID: "handshake-sync",
			Deltas:   response.CurrentState,
		}
		nm.crdt.Merge(batch)
	}

	// Atualiza lista de peers
	if len(response.PeerList) > 0 {
		nm.peerURLs = response.PeerList
	}

	return &response, nil
}

// HandleJoinRequest processa solicitação de entrada de novo nó
func (nm *NodeManager) HandleJoinRequest(req *HandshakeRequest) *HandshakeResponse {
	// Validações básicas
	if req.DroneID == "" {
		return &HandshakeResponse{
			Success: false,
			Message: "DroneID não pode estar vazio",
		}
	}

	if req.DroneID == nm.droneID {
		return &HandshakeResponse{
			Success: false,
			Message: "Não é possível conectar consigo mesmo",
		}
	}

	// Verifica se já conhece este peer
	peerURL := fmt.Sprintf("http://%s", req.DroneID)
	for _, existingPeer := range nm.peerURLs {
		if existingPeer == peerURL {
			return &HandshakeResponse{
				Success: false,
				Message: "Peer já conhecido na rede",
			}
		}
	}

	// Adiciona o novo peer
	nm.peerURLs = append(nm.peerURLs, peerURL)

	// Prepara resposta com estado atual
	currentState := nm.crdt.GetState()

	return &HandshakeResponse{
		Success:      true,
		Message:      fmt.Sprintf("Bem-vindo à rede, %s", req.DroneID),
		CurrentState: currentState,
		PeerList:     nm.peerURLs,
	}
}

// GetPeers retorna lista atual de peers
func (nm *NodeManager) GetPeers() []string {
	return append([]string{}, nm.peerURLs...)
}

// RemovePeer remove um peer da lista (para casos de falha)
func (nm *NodeManager) RemovePeer(peerURL string) {
	for i, peer := range nm.peerURLs {
		if peer == peerURL {
			nm.peerURLs = append(nm.peerURLs[:i], nm.peerURLs[i+1:]...)
			break
		}
	}
}

// AddPeer adiciona um peer à lista
func (nm *NodeManager) AddPeer(peerURL string) {
	// Verifica se já existe
	for _, peer := range nm.peerURLs {
		if peer == peerURL {
			return
		}
	}
	nm.peerURLs = append(nm.peerURLs, peerURL)
}
