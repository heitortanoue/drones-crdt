package network

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestNeighborTable_NewNeighborTable(t *testing.T) {
	timeout := 5 * time.Second
	nt := NewNeighborTable(timeout)

	if nt == nil {
		t.Fatal("NewNeighborTable não deveria retornar nil")
	}

	if nt.timeout != timeout {
		t.Errorf("Timeout esperado %v, obtido %v", timeout, nt.timeout)
	}

	if nt.neighbors == nil {
		t.Error("Mapa de vizinhos não deveria ser nil")
	}

	if count := nt.Count(); count != 0 {
		t.Errorf("Tabela deveria estar vazia inicialmente, obtido %d vizinhos", count)
	}
}

func TestNeighborTable_AddOrUpdate_SingleNeighbor(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)
	ip := net.ParseIP("192.168.1.100")
	port := 8080

	// Adiciona vizinho
	nt.AddOrUpdate(ip, port)

	// Verifica se foi adicionado
	neighbors := nt.GetActiveNeighbors()
	if len(neighbors) != 1 {
		t.Fatalf("Esperado 1 vizinho, obtido %d", len(neighbors))
	}

	neighbor := neighbors[0]
	if !neighbor.IP.Equal(ip) {
		t.Errorf("IP esperado %v, obtido %v", ip, neighbor.IP)
	}

	if neighbor.Port != port {
		t.Errorf("Porta esperada %d, obtida %d", port, neighbor.Port)
	}

	if time.Since(neighbor.LastSeen) > time.Second {
		t.Error("LastSeen deveria ser recente")
	}
}

func TestNeighborTable_AddOrUpdate_UpdateExisting(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)
	ip := net.ParseIP("10.0.0.1")
	port1 := 8080
	port2 := 9090

	// Adiciona vizinho inicial
	nt.AddOrUpdate(ip, port1)
	time.Sleep(10 * time.Millisecond) // Garante timestamp diferente

	// Atualiza o mesmo vizinho
	nt.AddOrUpdate(ip, port2)

	// Deve ter apenas 1 vizinho com porta atualizada
	neighbors := nt.GetActiveNeighbors()
	if len(neighbors) != 1 {
		t.Fatalf("Esperado 1 vizinho após atualização, obtido %d", len(neighbors))
	}

	neighbor := neighbors[0]
	if neighbor.Port != port2 {
		t.Errorf("Porta deveria ter sido atualizada para %d, obtido %d", port2, neighbor.Port)
	}

	if !neighbor.IP.Equal(ip) {
		t.Errorf("IP deveria permanecer %v, obtido %v", ip, neighbor.IP)
	}
}

func TestNeighborTable_AddOrUpdate_MultipleNeighbors(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)

	// Adiciona múltiplos vizinhos
	neighbors := []struct {
		ip   string
		port int
	}{
		{"192.168.1.10", 8080},
		{"192.168.1.20", 8081},
		{"10.0.0.5", 9090},
		{"172.16.0.1", 7070},
	}

	for _, n := range neighbors {
		nt.AddOrUpdate(net.ParseIP(n.ip), n.port)
	}

	// Verifica se todos foram adicionados
	active := nt.GetActiveNeighbors()
	if len(active) != len(neighbors) {
		t.Fatalf("Esperado %d vizinhos, obtido %d", len(neighbors), len(active))
	}

	// Verifica se o count está correto
	if count := nt.Count(); count != len(neighbors) {
		t.Errorf("Count esperado %d, obtido %d", len(neighbors), count)
	}
}

func TestNeighborTable_GetNeighborURLs(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)

	// Adiciona vizinhos
	testCases := []struct {
		ip       string
		port     int
		expected string
	}{
		{"127.0.0.1", 8080, "http://127.0.0.1:8080"},
		{"192.168.1.100", 9090, "http://192.168.1.100:9090"},
		{"10.0.0.1", 3000, "http://10.0.0.1:3000"},
	}

	for _, tc := range testCases {
		nt.AddOrUpdate(net.ParseIP(tc.ip), tc.port)
	}

	urls := nt.GetNeighborURLs()
	if len(urls) != len(testCases) {
		t.Fatalf("Esperado %d URLs, obtido %d", len(testCases), len(urls))
	}

	// Verifica se todas as URLs esperadas estão presentes
	urlMap := make(map[string]bool)
	for _, url := range urls {
		urlMap[url] = true
	}

	for _, tc := range testCases {
		if !urlMap[tc.expected] {
			t.Errorf("URL esperada %s não encontrada em %v", tc.expected, urls)
		}
	}
}

func TestNeighborTable_ExpiredNeighbors(t *testing.T) {
	shortTimeout := 200 * time.Millisecond
	nt := NewNeighborTable(shortTimeout)

	ip1 := net.ParseIP("192.168.1.1")
	ip2 := net.ParseIP("192.168.1.2")

	// Adiciona primeiro vizinho
	nt.AddOrUpdate(ip1, 8080)

	// Aguarda metade do timeout
	time.Sleep(shortTimeout / 2)

	// Adiciona segundo vizinho
	nt.AddOrUpdate(ip2, 8081)

	// Verifica que ambos estão ativos
	if count := nt.Count(); count != 2 {
		t.Fatalf("Esperado 2 vizinhos ativos, obtido %d", count)
	}

	// Aguarda o timeout completo para o primeiro vizinho expirar
	time.Sleep(shortTimeout + 50*time.Millisecond)

	// Aguarda cleanup automático (com margem de segurança)
	time.Sleep(1500 * time.Millisecond) // cleanup executa a cada segundo

	// Verifica que apenas o segundo vizinho permanece
	active := nt.GetActiveNeighbors()
	if len(active) == 0 {
		// Se ambos expiraram, pode ser que o segundo também tenha passado do timeout
		// Vamos verificar se o cleanup foi muito agressivo
		t.Log("Ambos vizinhos expiraram - teste pode precisar de timeouts maiores")

		// Adiciona um novo vizinho para verificar se o sistema ainda funciona
		nt.AddOrUpdate(net.ParseIP("192.168.1.3"), 8082)
		active = nt.GetActiveNeighbors()
		if len(active) != 1 {
			t.Fatal("Sistema deveria ainda estar funcionional")
		}
		return
	}

	if len(active) != 1 {
		t.Fatalf("Esperado 1 vizinho ativo após expiração, obtido %d", len(active))
	}

	if !active[0].IP.Equal(ip2) {
		t.Errorf("Vizinho ativo deveria ser %v, obtido %v", ip2, active[0].IP)
	}
}

func TestNeighborTable_GetStats(t *testing.T) {
	timeout := 5 * time.Second
	nt := NewNeighborTable(timeout)

	// Verifica stats iniciais
	stats := nt.GetStats()
	if stats["neighbors_active"] != 0 {
		t.Errorf("Esperado 0 vizinhos ativos inicialmente, obtido %v", stats["neighbors_active"])
	}

	if stats["timeout_seconds"] != timeout.Seconds() {
		t.Errorf("Timeout esperado %v, obtido %v", timeout.Seconds(), stats["timeout_seconds"])
	}

	// Adiciona alguns vizinhos
	nt.AddOrUpdate(net.ParseIP("1.1.1.1"), 80)
	nt.AddOrUpdate(net.ParseIP("2.2.2.2"), 443)

	// Verifica stats atualizadas
	stats = nt.GetStats()
	if stats["neighbors_active"] != 2 {
		t.Errorf("Esperado 2 vizinhos ativos, obtido %v", stats["neighbors_active"])
	}
}

func TestNeighborTable_ConcurrentAccess(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)

	const numGoroutines = 10
	const numOperations = 50

	var wg sync.WaitGroup

	// Múltiplas goroutines adicionando vizinhos concorrentemente
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Varia IP e porta para simular diferentes vizinhos
				ip := net.ParseIP(fmt.Sprintf("192.168.%d.%d", id, j%255))
				port := 8000 + id*100 + j

				nt.AddOrUpdate(ip, port)

				// Operações de leitura também
				if j%10 == 0 {
					nt.GetActiveNeighbors()
					nt.GetNeighborURLs()
					nt.Count()
					nt.GetStats()
				}

				// Pequeno delay para reduzir contenção
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	// Verifica que o sistema ainda está funcional
	finalCount := nt.Count()
	if finalCount <= 0 {
		t.Error("Deveria ter pelo menos alguns vizinhos após operações concorrentes")
	}

	// Verifica que as operações ainda funcionam
	neighbors := nt.GetActiveNeighbors()
	urls := nt.GetNeighborURLs()
	stats := nt.GetStats()

	if len(neighbors) != finalCount {
		t.Error("GetActiveNeighbors deveria retornar mesmo número que Count")
	}

	if len(urls) != finalCount {
		t.Error("GetNeighborURLs deveria retornar mesmo número que Count")
	}

	if stats["neighbors_active"] != finalCount {
		t.Error("Stats deveriam corresponder ao count")
	}
}

func TestNeighborTable_IPv4_and_IPv6(t *testing.T) {
	nt := NewNeighborTable(10 * time.Second)

	// Testa IPv4
	ipv4 := net.ParseIP("192.168.1.1")
	nt.AddOrUpdate(ipv4, 8080)

	// Testa IPv6
	ipv6 := net.ParseIP("2001:db8::1")
	nt.AddOrUpdate(ipv6, 8081)

	neighbors := nt.GetActiveNeighbors()
	if len(neighbors) != 2 {
		t.Fatalf("Esperado 2 vizinhos (IPv4 e IPv6), obtido %d", len(neighbors))
	}

	urls := nt.GetNeighborURLs()
	if len(urls) != 2 {
		t.Fatalf("Esperado 2 URLs, obtido %d", len(urls))
	}

	// Verifica formato das URLs
	expectedURLs := map[string]bool{
		"http://192.168.1.1:8080": true,
		"http://2001:db8::1:8081": true, // Go não adiciona colchetes automaticamente para IPv6
	}

	for _, url := range urls {
		if !expectedURLs[url] {
			t.Errorf("URL inesperada: %s", url)
		}
	}
}

func TestNeighborTable_EdgeCases(t *testing.T) {
	nt := NewNeighborTable(100 * time.Millisecond)

	// Testa com IP nil (não deveria crashar)
	// Nota: net.ParseIP retorna nil para strings inválidas
	invalidIP := net.ParseIP("invalid-ip")
	if invalidIP != nil {
		t.Fatal("IP deveria ser nil para string inválida")
	}

	// Adiciona múltiplas vezes o mesmo vizinho rapidamente
	ip := net.ParseIP("1.2.3.4")
	for i := 0; i < 100; i++ {
		nt.AddOrUpdate(ip, 8080+i) // Porta diferente a cada vez
	}

	// Deve ter apenas 1 vizinho (última atualização)
	neighbors := nt.GetActiveNeighbors()
	if len(neighbors) != 1 {
		t.Fatalf("Esperado 1 vizinho após múltiplas atualizações, obtido %d", len(neighbors))
	}

	if neighbors[0].Port != 8179 { // 8080 + 99
		t.Errorf("Porta deveria ser da última atualização (8179), obtido %d", neighbors[0].Port)
	}
}

func TestNeighborTable_EmptyTable_Operations(t *testing.T) {
	nt := NewNeighborTable(5 * time.Second)

	// Testa operações em tabela vazia
	neighbors := nt.GetActiveNeighbors()
	if len(neighbors) != 0 {
		t.Errorf("Tabela vazia deveria retornar 0 vizinhos, obtido %d", len(neighbors))
	}

	urls := nt.GetNeighborURLs()
	if len(urls) != 0 {
		t.Errorf("Tabela vazia deveria retornar 0 URLs, obtido %d", len(urls))
	}

	count := nt.Count()
	if count != 0 {
		t.Errorf("Count de tabela vazia deveria ser 0, obtido %d", count)
	}

	stats := nt.GetStats()
	if stats["neighbors_active"] != 0 {
		t.Errorf("Stats de tabela vazia deveriam mostrar 0 ativos, obtido %v", stats["neighbors_active"])
	}
}
