package swim

import (
	"fmt"
	"log"

	"github.com/hashicorp/memberlist"
)

// SwimEvents implementa EventDelegate para callback de eventos do memberlist
type SwimEvents struct {
	nodeID string
}

// NotifyJoin é chamado quando um nó se junta ao cluster
func (e *SwimEvents) NotifyJoin(n *memberlist.Node) {
	if n.Name != e.nodeID {
		log.Printf("[SWIM] Nó %s (%s) se juntou ao cluster", n.Name, n.Address())
	}
}

// NotifyLeave é chamado quando um nó deixa o cluster (gracefully)
func (e *SwimEvents) NotifyLeave(n *memberlist.Node) {
	log.Printf("[SWIM] Nó %s deixou o cluster", n.Name)
}

// NotifyUpdate é chamado quando metadados de um nó são atualizados
func (e *SwimEvents) NotifyUpdate(n *memberlist.Node) {
	log.Printf("[SWIM] Nó %s foi atualizado", n.Name)
}

// MembershipManager gerencia o memberlist e fornece interface simples
type MembershipManager struct {
	ml      *memberlist.Memberlist
	nodeID  string
	apiPort int // porta da API REST (diferente da porta SWIM)
}

// MembershipConfig configuração para criar o membership
type MembershipConfig struct {
	NodeID   string   // ID único do nó (ex: "drone-1")
	BindAddr string   // endereço para bind (ex: "0.0.0.0")
	BindPort int      // porta SWIM (padrão 7946)
	APIPort  int      // porta da API REST (ex: 8080)
	Seeds    []string // lista de seeds para join inicial
}

// NewMembershipManager cria um novo gerenciador de membership usando SWIM
func NewMembershipManager(config MembershipConfig) (*MembershipManager, error) {
	// Configuração padrão para LAN com timeouts otimizados
	cfg := memberlist.DefaultLANConfig()
	cfg.Name = config.NodeID
	cfg.BindAddr = config.BindAddr
	cfg.BindPort = config.BindPort

	// Configura eventos para logging
	cfg.Events = &SwimEvents{nodeID: config.NodeID}

	// Configurações adicionais para melhor performance em IoT
	cfg.PushPullInterval = 30000000000 // 30s (reduz tráfego)
	cfg.ProbeTimeout = 1000000000      // 1s
	cfg.ProbeInterval = 5000000000     // 5s

	// Cria o memberlist
	ml, err := memberlist.Create(cfg)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar memberlist: %v", err)
	}

	manager := &MembershipManager{
		ml:      ml,
		nodeID:  config.NodeID,
		apiPort: config.APIPort,
	}

	// Tenta se juntar aos seeds se fornecidos
	if len(config.Seeds) > 0 {
		// Filtra o próprio nó dos seeds
		validSeeds := make([]string, 0, len(config.Seeds))
		for _, seed := range config.Seeds {
			if seed != config.NodeID {
				validSeeds = append(validSeeds, seed)
			}
		}

		if len(validSeeds) > 0 {
			joinCount, err := ml.Join(validSeeds)
			if err != nil {
				log.Printf("[SWIM] Aviso: erro ao juntar-se aos seeds %v: %v", validSeeds, err)
			} else {
				log.Printf("[SWIM] Juntou-se a %d nós seeds", joinCount)
			}
		}
	}

	return manager, nil
}

// GetLiveMembers retorna lista de membros ativos (excluindo este nó)
func (m *MembershipManager) GetLiveMembers() []*memberlist.Node {
	allMembers := m.ml.Members()
	liveMembers := make([]*memberlist.Node, 0, len(allMembers))

	for _, member := range allMembers {
		if member.Name != m.nodeID {
			liveMembers = append(liveMembers, member)
		}
	}

	return liveMembers
}

// GetMemberURLs retorna URLs da API REST dos membros ativos
func (m *MembershipManager) GetMemberURLs() []string {
	members := m.GetLiveMembers()
	urls := make([]string, 0, len(members))

	for _, member := range members {
		// Assume que a API REST roda na porta configurada
		url := fmt.Sprintf("http://%s:%d", member.Addr.String(), m.apiPort)
		urls = append(urls, url)
	}

	return urls
}

// GetMemberCount retorna o número total de membros (incluindo este nó)
func (m *MembershipManager) GetMemberCount() int {
	return m.ml.NumMembers()
}

// GetNodeID retorna o ID deste nó
func (m *MembershipManager) GetNodeID() string {
	return m.nodeID
}

// GetLocalAddr retorna o endereço local do memberlist
func (m *MembershipManager) GetLocalAddr() string {
	return m.ml.LocalNode().Address()
}

// Leave faz este nó deixar o cluster gracefully
func (m *MembershipManager) Leave() error {
	err := m.ml.Leave(5000000000) // timeout 5s
	if err != nil {
		return fmt.Errorf("erro ao deixar o cluster: %v", err)
	}
	return nil
}

// Shutdown desliga o memberlist completamente
func (m *MembershipManager) Shutdown() error {
	err := m.ml.Shutdown()
	if err != nil {
		return fmt.Errorf("erro ao desligar memberlist: %v", err)
	}
	return nil
}

// GetStats retorna estatísticas do memberlist
func (m *MembershipManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["node_id"] = m.nodeID
	stats["total_members"] = m.ml.NumMembers()
	stats["live_members"] = len(m.GetLiveMembers())
	stats["local_addr"] = m.ml.LocalNode().Address()

	return stats
}

// JoinNode tenta adicionar um novo nó ao cluster
func (m *MembershipManager) JoinNode(nodeAddr string) error {
	joinCount, err := m.ml.Join([]string{nodeAddr})
	if err != nil {
		return fmt.Errorf("erro ao conectar ao nó %s: %v", nodeAddr, err)
	}

	log.Printf("[SWIM] Conectou-se a %d nós via %s", joinCount, nodeAddr)
	return nil
}
