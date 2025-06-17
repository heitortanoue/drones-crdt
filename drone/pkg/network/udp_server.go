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

	// Habilita configurações otimizadas para multicast
	if err := s.enableBroadcast(); err != nil {
		log.Printf("[UDP] Aviso: não foi possível otimizar para multicast: %v", err)
	}

	s.running = true
	log.Printf("[UDP] Servidor iniciado na porta %d (multicast Gappa habilitado)", s.port)

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

		// Evita adicionar o próprio drone como vizinho
		if addr.Port == s.port {
			continue
		}

		// Mapeia porta UDP para porta TCP (UDP 7000->TCP 8080, UDP 7001->TCP 8081, etc)
		tcpPort := 8080 + (addr.Port - 7000)
		s.neighborTable.AddOrUpdate(addr.IP, tcpPort)

		log.Printf("[UDP] Vizinho descoberto: %s (UDP:%d -> TCP:%d)", addr.IP.String(), addr.Port, tcpPort)

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

// Broadcast envia um pacote multicast seguindo protocolo Gappa
func (s *UDPServer) Broadcast(data []byte) {
	// Protocolo Gappa: usa apenas multicast para descoberta
	if err := s.MulticastGappa(data); err != nil {
		log.Printf("[UDP] Erro no multicast Gappa: %v", err)
	}
}

// MulticastGappa envia multicast seguindo protocolo Gappa (224.0.0.118)
func (s *UDPServer) MulticastGappa(data []byte) error {
	if s.conn == nil {
		return fmt.Errorf("servidor UDP não iniciado")
	}

	// Endereço multicast conforme protocolo Gappa
	multicastAddr := &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.118"),
		Port: s.port,
	}

	_, err := s.conn.WriteToUDP(data, multicastAddr)
	if err != nil {
		// Fallback: se multicast falhar, tenta broadcast local para desenvolvimento
		log.Printf("[UDP] Multicast falhou (%v), tentando fallback para broadcast local", err)
		return s.fallbackBroadcast(data)
	}

	log.Printf("[UDP] Multicast Gappa enviado para 224.0.0.118:%d (%d bytes)", s.port, len(data))
	return nil
}

// fallbackBroadcast implementa fallback para quando multicast não estiver disponível
func (s *UDPServer) fallbackBroadcast(data []byte) error {
	// Para desenvolvimento local, tenta algumas portas conhecidas
	localPorts := []int{7000, 7001, 7002, 7003, 7004}

	successCount := 0
	for _, port := range localPorts {
		if port == s.port {
			continue // Não envia para si mesmo
		}

		addr := &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: port,
		}

		_, err := s.conn.WriteToUDP(data, addr)
		if err == nil {
			successCount++
		}
	}

	log.Printf("[UDP] Fallback broadcast enviado para %d portas locais", successCount)
	return nil
}

// enableBroadcast habilita configurações otimizadas para multicast
func (s *UDPServer) enableBroadcast() error {
	if s.conn == nil {
		return fmt.Errorf("conexão UDP não iniciada")
	}

	// Define buffers para melhor performance de multicast
	if err := s.conn.SetWriteBuffer(64 * 1024); err != nil {
		log.Printf("[UDP] Aviso: erro ao definir buffer de escrita: %v", err)
	}

	if err := s.conn.SetReadBuffer(64 * 1024); err != nil {
		log.Printf("[UDP] Aviso: erro ao definir buffer de leitura: %v", err)
	}

	log.Printf("[UDP] Socket configurado para broadcast")
	return nil
}

// GetStats retorna estatísticas do servidor UDP
func (s *UDPServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"udp_port": s.port,
		"running":  s.running,
		"drone_id": s.droneID,
	}
}
