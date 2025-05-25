package gossip

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/heitortanoue/tcc/sensor"
)

/* ---------- mocks ---------- */

// mockMembership implementa só os métodos usados pelo PeerClient.
type mockMembership struct {
	urls  []string
	nodes []*memberlist.Node
	stats map[string]interface{}
}

func (m *mockMembership) GetMemberURLs() []string { return append([]string{}, m.urls...) }
func (m *mockMembership) GetLiveMembers() []*memberlist.Node {
	return append([]*memberlist.Node{}, m.nodes...)
}
func (m *mockMembership) GetStats() map[string]interface{} { return m.stats }

/* ---------- helpers ---------- */

func newCRDTWithPending(n int) *sensor.SensorCRDT {
	crdt := sensor.NewSensorCRDT("test")
	for i := 0; i < n; i++ {
		crdt.AddDelta(sensor.SensorReading{
			SensorID:  "s" + string(rune('A'+i)),
			Timestamp: time.Now().UnixMilli(),
			Value:     10 + float64(i),
		})
	}
	return crdt
}

/* ---------- tests ---------- */

func TestGossipToPeers(t *testing.T) {
	// peer fake sempre devolve 200 OK
	postCount := 0
	peerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delta" && r.Method == http.MethodPost {
			postCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer peerSrv.Close()

	crdt := newCRDTWithPending(3)
	member := &mockMembership{
		urls:  []string{peerSrv.URL},
		nodes: []*memberlist.Node{}, // lista vazia por simplicidade
	}
	cli := NewPeerClient("d1", crdt, member)

	cli.gossipToPeers() // usa função não-exportada mas visível dentro do mesmo pacote

	if postCount != 1 {
		t.Fatalf("esperado 1 POST /delta, obtido %d", postCount)
	}
	if got := len(crdt.GetPendingDeltas()); got != 0 {
		t.Fatalf("pending não foi limpo, restam %d deltas", got)
	}
}

func TestPullFromPeer(t *testing.T) {
	rem := []sensor.SensorDelta{{
		DroneID:   "remote",
		SensorID:  "sensorX",
		Timestamp: time.Now().UnixMilli(),
		Value:     42,
	}}
	peerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/deltas" {
			json.NewEncoder(w).Encode(struct {
				Pending []sensor.SensorDelta `json:"pending"`
			}{Pending: rem})
			return
		}
		http.NotFound(w, r)
	}))
	defer peerSrv.Close()

	crdt := sensor.NewSensorCRDT("test")
	member := &mockMembership{urls: []string{peerSrv.URL}, nodes: []*memberlist.Node{}}
	cli := NewPeerClient("d1", crdt, member)

	if err := cli.PullFromPeer(peerSrv.URL); err != nil {
		t.Fatalf("PullFromPeer erro: %v", err)
	}
	state := crdt.GetState()
	if len(state) != 1 || state[0].SensorID != "sensorX" || state[0].Value != 42 {
		t.Fatalf("estado inesperado %+v", state)
	}
}

func TestPeerClientGetters(t *testing.T) {
	member := &mockMembership{
		urls:  []string{"http://a", "http://b"},
		nodes: []*memberlist.Node{},
		stats: map[string]interface{}{"alive": 2},
	}
	cli := NewPeerClient("d1", sensor.NewSensorCRDT("test"), member)

	if !reflect.DeepEqual(cli.GetPeerURLs(), member.urls) {
		t.Fatalf("GetPeerURLs não casa com membership")
	}
	if cli.GetActivePeerCount() != 0 { // mudou para 0 pois nodes está vazio
		t.Fatalf("esperado 0 ativos, obtido %d", cli.GetActivePeerCount())
	}
	if !reflect.DeepEqual(cli.GetMembershipStats(), member.stats) {
		t.Fatalf("stats divergente")
	}
}
