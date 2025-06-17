package network

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
)

// MessageProcessor interface para processar mensagens de controle
type MessageProcessor interface {
	ProcessMessage(data []byte, senderIP string)
}

// UDPServer gerencia comunicação UDP na porta 7000 (canal de controle)
type UDPServer struct {
	conn             *net.UDPConn
	neighborTable    *NeighborTable
	messageProcessor MessageProcessor // Fase 3: Processador de mensagens de controle
	droneID          string
	port             int
	running          bool
}

// NewUDPServer cria um novo servidor UDP
func NewUDPServer(droneID string, port int, neighborTable *NeighborTable) *UDPServer {
	return &UDPServer{
		droneID:       droneID,
		port:          port,
		neighborTable: neighborTable,
		running:       false,
	}
}

// SetMessageProcessor define o processador de mensagens de controle
func (s *UDPServer) SetMessageProcessor(processor MessageProcessor) {
	s.messageProcessor = processor
}

// Start inicia o servidor UDP
func (s *UDPServer) Start() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("erro ao resolver endereço UDP: %v", err)
	}

	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("erro ao iniciar servidor UDP: %v", err)
	}

	s.running = true
	log.Printf("[UDP] Servidor iniciado na porta %d", s.port)

	go s.handleIncomingPackets()
	return nil
}

// Stop para o servidor UDP
func (s *UDPServer) Stop() error {
	s.running = false
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// handleIncomingPackets processa pacotes UDP recebidos
func (s *UDPServer) handleIncomingPackets() {
	buffer := make([]byte, 1024)

	for s.running {
		n, addr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if s.running {
				log.Printf("[UDP] Erro ao ler pacote: %v", err)
			}
			continue
		}

		// Qualquer pacote recebido atualiza a tabela de vizinhos
		// Por enquanto, assumimos porta TCP padrão 8080
		s.neighborTable.AddOrUpdate(addr.IP, 8080)

		// Processa o conteúdo do pacote
		go s.processPacket(buffer[:n], addr)
	}
}

// processPacket processa um pacote UDP específico
func (s *UDPServer) processPacket(data []byte, addr *net.UDPAddr) {
	log.Printf("[UDP] Pacote recebido de %s: %d bytes", addr.IP.String(), len(data))

	// Fase 3: Integração com sistema de controle
	if s.messageProcessor != nil {
		s.messageProcessor.ProcessMessage(data, addr.IP.String())
	} else {
		// Fallback: tenta decodificar como JSON para debug
		var msg interface{}
		if err := json.Unmarshal(data, &msg); err == nil {
			log.Printf("[UDP] Conteúdo: %+v", msg)
		}
	}
}

// SendPacket envia um pacote UDP para um endereço específico
func (s *UDPServer) SendPacket(data []byte, targetIP net.IP, targetPort int) error {
	if s.conn == nil {
		return fmt.Errorf("servidor UDP não iniciado")
	}

	addr := &net.UDPAddr{
		IP:   targetIP,
		Port: targetPort,
	}

	_, err := s.conn.WriteToUDP(data, addr)
	if err != nil {
		return fmt.Errorf("erro ao enviar pacote UDP: %v", err)
	}

	log.Printf("[UDP] Pacote enviado para %s:%d (%d bytes)", targetIP.String(), targetPort, len(data))
	return nil
}

// SendTo implementa interface UDPSender - envia para IP específico
func (s *UDPServer) SendTo(data []byte, targetIP string, targetPort int) error {
	ip := net.ParseIP(targetIP)
	if ip == nil {
		return fmt.Errorf("IP inválido: %s", targetIP)
	}

	return s.SendPacket(data, ip, targetPort)
}

// Broadcast envia um pacote para todos os vizinhos ativos
func (s *UDPServer) Broadcast(data []byte) {
	neighbors := s.neighborTable.GetActiveNeighbors()

	for _, neighbor := range neighbors {
		if err := s.SendPacket(data, neighbor.IP, s.port); err != nil {
			log.Printf("[UDP] Erro ao enviar para %s: %v", neighbor.IP.String(), err)
		}
	}
}

// GetStats retorna estatísticas do servidor UDP
func (s *UDPServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"udp_port": s.port,
		"running":  s.running,
		"drone_id": s.droneID,
	}
}
