package network

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/heitortanoue/tcc/pkg/protocol"
	"golang.org/x/net/ipv4"
)

// getLocalIP detects the real container IP (not loopback)
func (s *UDPServer) getLocalIP() (net.IP, error) {
	// Try connecting to an external address to discover the local IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, fmt.Errorf("failed to detect local IP: %v", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// UDPServer manages UDP communication on port 7000 (control channel)
type UDPServer struct {
	conn          *net.UDPConn
	neighborTable *NeighborTable
	droneID       string
	port          int
	running       bool
	localIP       net.IP
}

const MULTICAST_IP = "224.0.0.118" // Multicast group address

// NewUDPServer creates a new UDP server
func NewUDPServer(droneID string, port int, neighborTable *NeighborTable) *UDPServer {
	return &UDPServer{
		droneID:       droneID,
		port:          port,
		neighborTable: neighborTable,
		running:       false,
	}
}

// Start launches the UDP server
func (s *UDPServer) Start() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP server: %v", err)
	}

	// Detect the real container IP (not :: or 0.0.0.0)
	s.localIP, err = s.getLocalIP()
	if err != nil {
		log.Printf("[UDP] Warning: failed to detect local IP: %v", err)
		// Fallback: use the IP from the UDP connection
		if la, ok := s.conn.LocalAddr().(*net.UDPAddr); ok {
			s.localIP = la.IP
		}
	}
	log.Printf("[UDP] Local IP detected: %s", s.localIP.String())

	// Configure multicast
	if err := s.setupMulticast(); err != nil {
		s.conn.Close()
		return fmt.Errorf("failed to configure multicast: %v", err)
	}

	// Enable optimized settings for multicast
	if err := s.enableBroadcast(); err != nil {
		log.Fatalf("[UDP] ERROR: failed to optimize for multicast: %v", err)
	}

	s.running = true
	log.Printf("[UDP] Server started on port %d (multicast enabled)", s.port)

	go s.handleIncomingPackets()
	return nil
}

// Stop shuts down the UDP server
func (s *UDPServer) Stop() error {
	s.running = false
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// handleIncomingPackets processes received UDP packets
func (s *UDPServer) handleIncomingPackets() {
	buffer := make([]byte, 2048) // Increased buffer size for larger packets

	for s.running {
		n, addr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			if s.running {
				log.Printf("[UDP] Error reading packet: %v", err)
			}
			continue
		}

		// Ignore packets sent by the same drone
		if addr.IP.Equal(s.localIP) {
			continue
		}

		log.Printf("[UDP] Packet received from %s:%d (%d bytes)", addr.IP.String(), addr.Port, n)

		// Process the packet contents
		go s.processPacket(buffer[:n], addr)
	}
}

// processPacket handles a specific UDP packet
func (s *UDPServer) processPacket(data []byte, addr *net.UDPAddr) {
	var helloMsg = protocol.HelloMessage{}

	// Process HELLO message
	if err := json.Unmarshal(data, &helloMsg); err == nil && helloMsg.ID != "" {
		// Update neighborTable with HELLO message information
		s.neighborTable.AddOrUpdate(helloMsg, addr.IP, 8080) // Fixed TCP port 8080
		log.Printf("[UDP] Neighbor discovered via HELLO: %s (TCP:8080)", addr.IP.String())
		return
	}

	// If not a valid HELLO message, just log it
	log.Printf("[UDP] Packet received is not a valid HELLO message")
}

// SendPacket sends a UDP packet to a specific address
func (s *UDPServer) SendPacket(data []byte, targetIP net.IP, targetPort int) error {
	if s.conn == nil {
		return fmt.Errorf("UDP server not started")
	}

	addr := &net.UDPAddr{
		IP:   targetIP,
		Port: targetPort,
	}

	_, err := s.conn.WriteToUDP(data, addr)
	if err != nil {
		return fmt.Errorf("failed to send UDP packet: %v", err)
	}

	log.Printf("[UDP] Packet sent to %s:%d (%d bytes)", targetIP.String(), targetPort, len(data))
	return nil
}

// SendTo implements UDPSender interface - sends to a specific IP
func (s *UDPServer) SendTo(data []byte, targetIP string, targetPort int) error {
	ip := net.ParseIP(targetIP)
	if ip == nil {
		return fmt.Errorf("invalid IP: %s", targetIP)
	}

	return s.SendPacket(data, ip, targetPort)
}

// Broadcast sends exclusively via multicast.
// If sending fails, it only logs the error (no fallback).
func (s *UDPServer) Broadcast(data []byte) {
	if err := s.Multicast(data); err != nil {
		log.Printf("[UDP] ERROR multicast: %v (no fallback applied)", err)
	}
}

// Multicast sends a packet to the multicast group
func (s *UDPServer) Multicast(data []byte) error {
	if s.conn == nil {
		return fmt.Errorf("UDP server not started")
	}

	multicastAddr := &net.UDPAddr{
		IP:   net.ParseIP(MULTICAST_IP),
		Port: s.port,
	}

	_, err := s.conn.WriteToUDP(data, multicastAddr)
	if err != nil {
		return fmt.Errorf("multicast send failed: %v", err)
	}
	return nil
}

// enableBroadcast enables optimized socket settings for multicast
func (s *UDPServer) enableBroadcast() error {
	if s.conn == nil {
		return fmt.Errorf("UDP connection not started")
	}

	if err := s.conn.SetWriteBuffer(64 * 1024); err != nil {
		log.Printf("[UDP] Warning: failed to set write buffer: %v", err)
	}

	if err := s.conn.SetReadBuffer(64 * 1024); err != nil {
		log.Printf("[UDP] Warning: failed to set read buffer: %v", err)
	}

	log.Printf("[UDP] Socket configured for broadcast")
	return nil
}

// setupMulticast configures the server to receive multicast packets
func (s *UDPServer) setupMulticast() error {
	multicastGroup := net.ParseIP(MULTICAST_IP)
	if multicastGroup == nil {
		return fmt.Errorf("invalid multicast address")
	}

	// Get the default network interface (Docker usually uses eth0)
	intf, err := net.InterfaceByName("eth0")
	if err != nil {
		// Fallback: pick the first available non-loopback multicast interface
		interfaces, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("failed to get network interfaces: %v", err)
		}

		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 &&
				iface.Flags&net.FlagLoopback == 0 &&
				iface.Flags&net.FlagMulticast != 0 {
				intf = &iface
				log.Printf("[UDP] Using network interface: %s", iface.Name)
				break
			}
		}

		if intf == nil {
			return fmt.Errorf("no valid network interface found")
		}
	} else {
		log.Printf("[UDP] Using eth0 interface for multicast")
	}

	// Create an IPv4 PacketConn for multicast
	packetConn := ipv4.NewPacketConn(s.conn)

	// Join the multicast group
	if err := packetConn.JoinGroup(intf, &net.UDPAddr{IP: multicastGroup, Port: s.port}); err != nil {
		return fmt.Errorf("failed to join multicast group: %v", err)
	}

	// Configure to receive multicast packets
	if err := packetConn.SetMulticastInterface(intf); err != nil {
		log.Printf("[UDP] Warning: failed to set multicast interface: %v", err)
	}

	if err := s.conn.SetReadBuffer(65536); err != nil {
		log.Printf("[UDP] Warning: failed to set read buffer: %v", err)
	}

	log.Printf("[UDP] Joined multicast group on interface %s", intf.Name)
	return nil
}

// GetStats returns UDP server statistics
func (s *UDPServer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"udp_port": s.port,
		"running":  s.running,
		"drone_id": s.droneID,
	}
}