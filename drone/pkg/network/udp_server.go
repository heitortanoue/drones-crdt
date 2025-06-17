// UDPServer implements Gappa canal‑0 control using **multicast only**; no broadcast fallback.
package network

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"golang.org/x/net/ipv4"
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
	localIP          net.IP
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

	// guarda IP local da conexão para filtrar pacotes próprios
	if la, ok := s.conn.LocalAddr().(*net.UDPAddr); ok {
		s.localIP = la.IP
	}

	// Configura multicast Gappa
	if err := s.setupMulticast(); err != nil {
		s.conn.Close()
		return fmt.Errorf("erro ao configurar multicast: %v", err)
	}

	// Habilita configurações otimizadas para multicast
	if err := s.enableBroadcast(); err != nil {
		log.Fatalf("[UDP] ERRO: não foi possível otimizar para multicast: %v", err)
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
	buffer := make([]byte, 2048) // Aumentado para comportar pacotes maiores

	for s.running {
		n, addr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if s.running {
				log.Printf("[UDP] Erro ao ler pacote: %v", err)
			}
			continue
		}

		// Evita adicionar o próprio drone como vizinho
		if addr.Port == s.port && addr.IP.Equal(s.localIP) {
			log.Printf("[UDP] Ignorando pacote de %s:%d (próprio drone)", addr.IP.String(), addr.Port)
			continue
		}

		log.Printf("[UDP] Pacote recebido de %s:%d (%d bytes)", addr.IP.String(), addr.Port, n)

		// Atualiza neighborTable com porta TCP fixa 8080
		s.neighborTable.AddOrUpdate(addr.IP, 8080)

		log.Printf("[UDP] Vizinho descoberto: %s (UDP:%d)", addr.IP.String(), addr.Port)

		// Processa o conteúdo do pacote
		go s.processPacket(buffer[:n], addr)
	}
}

// processPacket processa um pacote UDP específico
func (s *UDPServer) processPacket(data []byte, addr *net.UDPAddr) {
	log.Printf("[UDP] Processando pacote de %s:%d (%d bytes)", addr.IP.String(), addr.Port, len(data))

	// Fase 3: Integração com sistema de controle
	if s.messageProcessor != nil {
		log.Printf("[UDP] Enviando para messageProcessor...")
		s.messageProcessor.ProcessMessage(data, addr.IP.String())
	} else {
		log.Printf("[UDP] Nenhum messageProcessor configurado, tentando decode JSON...")
		// Fallback: tenta decodificar como JSON para debug
		var msg interface{}
		if err := json.Unmarshal(data, &msg); err == nil {
			log.Printf("[UDP] Conteúdo JSON: %+v", msg)
		} else {
			log.Printf("[UDP] Não é JSON válido, dados raw: %s", string(data))
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

// Broadcast envia exclusivamente via multicast Gappa.
// Se o envio falhar, apenas registra o erro (sem fallback).
func (s *UDPServer) Broadcast(data []byte) {
	if err := s.MulticastGappa(data); err != nil {
		log.Printf("[UDP] ERRO multicast: %v (nenhum fallback aplicado)", err)
	}
}

// MulticastGappa envia multicast seguindo protocolo Gappa (224.0.0.118)
func (s *UDPServer) MulticastGappa(data []byte) error {
	if s.conn == nil {
		return fmt.Errorf("servidor UDP não iniciado")
	}

	multicastAddr := &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.118"),
		Port: s.port,
	}

	_, err := s.conn.WriteToUDP(data, multicastAddr)
	if err != nil {
		return fmt.Errorf("falha no envio multicast: %v", err)
	}

	log.Printf("[UDP] Multicast Gappa enviado (%d bytes)", len(data))
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

// setupMulticast configura o servidor para receber pacotes multicast do grupo Gappa
func (s *UDPServer) setupMulticast() error {
	// Endereço do grupo multicast Gappa
	multicastGroup := net.ParseIP("224.0.0.118")
	if multicastGroup == nil {
		return fmt.Errorf("endereço multicast inválido")
	}

	// Obtém a interface de rede padrão
	intf, err := net.InterfaceByName("eth0")
	if err != nil {
		// Fallback: usa a primeira interface disponível
		interfaces, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("erro ao obter interfaces de rede: %v", err)
		}

		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				intf = &iface
				break
			}
		}

		if intf == nil {
			return fmt.Errorf("nenhuma interface de rede válida encontrada")
		}
	}

	// Cria um PacketConn IPv4 para multicast
	packetConn := ipv4.NewPacketConn(s.conn)

	// Junta-se ao grupo multicast
	if err := packetConn.JoinGroup(intf, &net.UDPAddr{IP: multicastGroup, Port: s.port}); err != nil {
		return fmt.Errorf("erro ao entrar no grupo multicast: %v", err)
	}

	// Configura para receber pacotes multicast
	if err := packetConn.SetMulticastInterface(intf); err != nil {
		log.Printf("[UDP] Aviso: erro ao definir interface multicast: %v", err)
	}

	// Aumenta buffer de leitura na conexão UDP original
	if err := s.conn.SetReadBuffer(65536); err != nil {
		log.Printf("[UDP] Aviso: erro ao definir buffer de leitura: %v", err)
	}

	log.Printf("[UDP] Entrou no grupo multicast 224.0.0.118 na interface %s", intf.Name)
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
