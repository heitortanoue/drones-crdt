package network

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/heitortanoue/tcc/pkg/protocol"
	"golang.org/x/net/ipv4"
)

// getLocalIP detecta o IP real do container (não loopback)
func (s *UDPServer) getLocalIP() (net.IP, error) {
	// Tenta conectar a um endereço externo para descobrir o IP local
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, fmt.Errorf("erro ao detectar IP local: %v", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// UDPServer gerencia comunicação UDP na porta 7000 (canal de controle)
type UDPServer struct {
	conn          *net.UDPConn
	neighborTable *NeighborTable
	droneID       string
	port          int
	running       bool
	localIP       net.IP
}

const MULTICAST_IP = "224.0.0.118" // IP multicast

// NewUDPServer cria um novo servidor UDP
func NewUDPServer(droneID string, port int, neighborTable *NeighborTable) *UDPServer {
	return &UDPServer{
		droneID:       droneID,
		port:          port,
		neighborTable: neighborTable,
		running:       false,
	}
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

	// Detecta o IP real do container (não :: ou 0.0.0.0)
	s.localIP, err = s.getLocalIP()
	if err != nil {
		log.Printf("[UDP] Aviso: erro ao detectar IP local: %v", err)
		// Fallback: usa IP da conexão UDP
		if la, ok := s.conn.LocalAddr().(*net.UDPAddr); ok {
			s.localIP = la.IP
		}
	}
	log.Printf("[UDP] IP local detectado: %s", s.localIP.String())

	// Configura multicast
	if err := s.setupMulticast(); err != nil {
		s.conn.Close()
		return fmt.Errorf("erro ao configurar multicast: %v", err)
	}

	// Habilita configurações otimizadas para multicast
	if err := s.enableBroadcast(); err != nil {
		log.Fatalf("[UDP] ERRO: não foi possível otimizar para multicast: %v", err)
	}

	s.running = true
	log.Printf("[UDP] Servidor iniciado na porta %d (multicast habilitado)", s.port)

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

		// Ignora pacotes enviados pelo próprio drone
		if addr.IP.Equal(s.localIP) {
			continue
		}

		log.Printf("[UDP] Pacote recebido de %s:%d (%d bytes)", addr.IP.String(), addr.Port, n)

		// Processa o conteúdo do pacote
		go s.processPacket(buffer[:n], addr)
	}
}

// processPacket processa um pacote UDP específico
func (s *UDPServer) processPacket(data []byte, addr *net.UDPAddr) {
	log.Printf("[UDP] Processando pacote de %s:%d (%d bytes)", addr.IP.String(), addr.Port, len(data))

	// Processa mensagem HELLO
	var helloMsg = protocol.HelloMessage{}

	if err := json.Unmarshal(data, &helloMsg); err == nil && helloMsg.ID != "" {
		log.Printf("[UDP] Mensagem HELLO recebida de %s: ID=%s, Version=%d",
			addr.IP.String(), helloMsg.ID, helloMsg.Version)

		// Atualiza neighborTable com informações da mensagem HELLO
		s.neighborTable.AddOrUpdate(helloMsg, addr.IP, addr.Port)
		log.Printf("[UDP] Vizinho descoberto via HELLO: %s (TCP:8080, Version:%d)",
			addr.IP.String(), helloMsg.Version)
		return
	}

	// Se não é uma mensagem HELLO válida, apenas registra
	log.Printf("[UDP] Pacote recebido não é uma mensagem HELLO válida")
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

// Broadcast envia exclusivamente via multicast.
// Se o envio falhar, apenas registra o erro (sem fallback).
func (s *UDPServer) Broadcast(data []byte) {
	if err := s.Multicast(data); err != nil {
		log.Printf("[UDP] ERRO multicast: %v (nenhum fallback aplicado)", err)
	}
}

func (s *UDPServer) Multicast(data []byte) error {
	if s.conn == nil {
		return fmt.Errorf("servidor UDP não iniciado")
	}

	multicastAddr := &net.UDPAddr{
		IP:   net.ParseIP(MULTICAST_IP),
		Port: s.port,
	}

	_, err := s.conn.WriteToUDP(data, multicastAddr)
	if err != nil {
		return fmt.Errorf("falha no envio multicast: %v", err)
	}

	log.Printf("[UDP] Multicast enviado (%d bytes)", len(data))
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

// setupMulticast configura o servidor para receber pacotes multicast
func (s *UDPServer) setupMulticast() error {
	multicastGroup := net.ParseIP(MULTICAST_IP)
	if multicastGroup == nil {
		return fmt.Errorf("endereço multicast inválido")
	}

	// Obtém a interface de rede padrão (Docker usa eth0)
	intf, err := net.InterfaceByName("eth0")
	if err != nil {
		// Fallback: usa a primeira interface disponível que não seja loopback
		interfaces, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("erro ao obter interfaces de rede: %v", err)
		}

		for _, iface := range interfaces {
			// Busca interface ativa, não-loopback e que suporte multicast
			if iface.Flags&net.FlagUp != 0 &&
				iface.Flags&net.FlagLoopback == 0 &&
				iface.Flags&net.FlagMulticast != 0 {
				intf = &iface
				log.Printf("[UDP] Usando interface de rede: %s", iface.Name)
				break
			}
		}

		if intf == nil {
			return fmt.Errorf("nenhuma interface de rede válida encontrada")
		}
	} else {
		log.Printf("[UDP] Usando interface eth0 para multicast")
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

	log.Printf("[UDP] Entrou no grupo multicast na interface %s", intf.Name)
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
