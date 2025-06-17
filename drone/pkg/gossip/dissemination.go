package gossip

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

// DeltaMsg representa uma mensagem de delta com TTL para disseminação
type DeltaMsg struct {
	ID        uuid.UUID          `json:"id"`
	TTL       int                `json:"ttl"`
	Data      sensor.SensorDelta `json:"data"`
	SenderID  string             `json:"sender_id"`
	Timestamp int64              `json:"timestamp"`
}

// DisseminationSystem gerencia disseminação TTL (Requisito F4)
type DisseminationSystem struct {
	droneID    string
	fanout     int // Fan-out fixo para 3 vizinhos
	defaultTTL int // TTL inicial padrão

	// Interfaces para comunicação
	neighborGetter NeighborGetter
	tcpSender      TCPSender
	cache          *DeduplicationCache

	// Controle de execução
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex

	// Métricas
	sentCount     int64
	receivedCount int64
	droppedCount  int64 // Por TTL=0 ou duplicatas
}

// NeighborGetter interface para obter vizinhos
type NeighborGetter interface {
	GetNeighborURLs() []string
	Count() int
}

// TCPSender interface para envio TCP
type TCPSender interface {
	SendDelta(url string, delta DeltaMsg) error
}

// NewDisseminationSystem cria um novo sistema de disseminação
func NewDisseminationSystem(droneID string, fanout, defaultTTL int, neighborGetter NeighborGetter, tcpSender TCPSender) *DisseminationSystem {
	return &DisseminationSystem{
		droneID:        droneID,
		fanout:         fanout,
		defaultTTL:     defaultTTL,
		neighborGetter: neighborGetter,
		tcpSender:      tcpSender,
		cache:          NewDeduplicationCache(10000), // Cache de 10k IDs
		running:        false,
		stopCh:         make(chan struct{}),
	}
}

// Start inicia o sistema de disseminação
func (ds *DisseminationSystem) Start() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	if ds.running {
		return
	}

	ds.running = true
	log.Printf("[DISSEMINATION] Iniciando sistema de disseminação para %s (fanout: %d, TTL: %d)",
		ds.droneID, ds.fanout, ds.defaultTTL)
}

// Stop para o sistema de disseminação
func (ds *DisseminationSystem) Stop() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	if !ds.running {
		return
	}

	ds.running = false
	close(ds.stopCh)
	log.Printf("[DISSEMINATION] Parando sistema de disseminação para %s", ds.droneID)
}

// DisseminateDelta dissemina um delta para vizinhos com TTL
func (ds *DisseminationSystem) DisseminateDelta(delta sensor.SensorDelta) error {
	ds.mutex.RLock()
	if !ds.running {
		ds.mutex.RUnlock()
		return nil
	}
	ds.mutex.RUnlock()

	// Cria mensagem com TTL inicial
	msg := DeltaMsg{
		ID:        delta.ID,
		TTL:       ds.defaultTTL,
		Data:      delta,
		SenderID:  ds.droneID,
		Timestamp: time.Now().UnixMilli(),
	}

	return ds.forwardDelta(msg)
}

// ProcessReceivedDelta processa delta recebido de outro nó
func (ds *DisseminationSystem) ProcessReceivedDelta(msg DeltaMsg) error {
	ds.mutex.Lock()
	ds.receivedCount++
	ds.mutex.Unlock()

	// Verifica deduplicação
	if ds.cache.Contains(msg.ID) {
		ds.mutex.Lock()
		ds.droppedCount++
		ds.mutex.Unlock()
		log.Printf("[DISSEMINATION] Delta %s descartado (duplicata)", msg.ID.String()[:8])
		return nil
	}

	// Adiciona ao cache
	ds.cache.Add(msg.ID)

	// Verifica TTL
	if msg.TTL <= 0 {
		ds.mutex.Lock()
		ds.droppedCount++
		ds.mutex.Unlock()
		log.Printf("[DISSEMINATION] Delta %s descartado (TTL=0)", msg.ID.String()[:8])
		return nil
	}

	log.Printf("[DISSEMINATION] Processando delta %s (TTL: %d)", msg.ID.String()[:8], msg.TTL)

	// Decrementa TTL e continua disseminação
	msg.TTL--
	msg.SenderID = ds.droneID // Atualiza sender para este nó

	return ds.forwardDelta(msg)
}

// forwardDelta envia delta para até 'fanout' vizinhos
func (ds *DisseminationSystem) forwardDelta(msg DeltaMsg) error {
	neighbors := ds.neighborGetter.GetNeighborURLs()
	if len(neighbors) == 0 {
		log.Printf("[DISSEMINATION] Nenhum vizinho disponível para delta %s", msg.ID.String()[:8])
		return nil
	}

	// Limita ao fanout configurado
	targetCount := ds.fanout
	if len(neighbors) < targetCount {
		targetCount = len(neighbors)
	}

	// Seleciona vizinhos aleatoriamente (estratégia simples)
	targets := selectRandomNeighbors(neighbors, targetCount)

	var errors []error
	successCount := 0

	for _, url := range targets {
		if err := ds.tcpSender.SendDelta(url, msg); err != nil {
			log.Printf("[DISSEMINATION] Erro ao enviar delta %s para %s: %v",
				msg.ID.String()[:8], url, err)
			errors = append(errors, err)
		} else {
			successCount++
		}
	}

	ds.mutex.Lock()
	ds.sentCount += int64(successCount)
	ds.mutex.Unlock()

	log.Printf("[DISSEMINATION] Delta %s enviado para %d/%d vizinhos",
		msg.ID.String()[:8], successCount, len(targets))

	if len(errors) > 0 {
		return errors[0] // Retorna primeiro erro
	}

	return nil
}

// selectRandomNeighbors seleciona até 'count' vizinhos aleatoriamente
func selectRandomNeighbors(neighbors []string, count int) []string {
	if len(neighbors) <= count {
		return neighbors
	}

	// Cria uma cópia para não modificar o original
	shuffled := make([]string, len(neighbors))
	copy(shuffled, neighbors)

	// Fisher-Yates shuffle simples
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:count]
}

// GetStats retorna estatísticas do sistema de disseminação
func (ds *DisseminationSystem) GetStats() map[string]interface{} {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return map[string]interface{}{
		"running":        ds.running,
		"fanout":         ds.fanout,
		"default_ttl":    ds.defaultTTL,
		"sent_count":     ds.sentCount,
		"received_count": ds.receivedCount,
		"dropped_count":  ds.droppedCount,
		"cache_size":     ds.cache.Size(),
		"neighbor_count": ds.neighborGetter.Count(),
	}
}

// IsRunning retorna se o sistema está executando
func (ds *DisseminationSystem) IsRunning() bool {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return ds.running
}
