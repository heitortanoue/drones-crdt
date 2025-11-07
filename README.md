# UAV Swarm Gossip + Delta-CRDT (Go)

A lightweight data plane for UAV swarms that keeps a shared state eventually consistent over fast-changing, intermittently connected FANETs.
It pairs delta state-based CRDTs (Delta-CRDTs) with push gossip and periodic anti-entropy repair. Discovery runs over UDP multicast; replication uses a small TCP/HTTP API. Written in Go for simplicity, observability, and speed.

What it does (in short)
	•	Neighbor discovery: UDP multicast HELLO beacons with jitter to track nearby drones under mobility.
	•	Replication: Delta-CRDT (Add-Wins Observed-Remove Set with dotted causal metadata) for order/loss/duplication tolerance and deterministic convergence.
	•	Dissemination: Epidemic push gossip (fan-out + TTL) for fast spread; anti-entropy for periodic full-state repair.
	•	Concurrency model: Each subsystem runs in its own goroutine; non-blocking timers and I/O.
	•	HTTP API: Inspect state, post deltas, inject sensor readings, and drive positions (for the simulator).

## Repository layout

```
├─ drone/                  # UAV node implementation
│  ├─ internal/
│  │  └─ config            # Runtime configuration parsing/defaults
│  └─ pkg/
│     ├─ crdt              # Delta AWOR-Set (+ dots, clocks, dot-cloud)
│     ├─ state             # Application state built on CRDT replicas
│     ├─ gossip            # Push-gossip + anti-entropy orchestration
│     ├─ network           # UDP HELLOs + TCP/HTTP server/client
│     ├─ protocol          # Message schemas & serialization
│     └─ sensor            # Toggle-based fire event generator
└─ simulator/              # Mininet-WiFi harness & scenario driver
```

## Quick start

Requires Go 1.24.2 or newer.

### Build
cd drone
go build ./...

### Run a node (example flags; see -h for all)
```
./drone \
  --id=drone-1 \
  --udp-port=7000 \
  --tcp-port=8080 \
  --bind=0.0.0.0 \
  --hello-interval=1s --hello-jitter=200ms \
  --delta-interval=3s --anti-entropy=60s \
  --fanout=3 --ttl=4 \
  --grid-size=1650x1650 \
  --sample-interval=50ms --confidence-threshold=0.5
```

Multicast discovery uses 224.0.0.118 on the chosen UDP port. Run multiple nodes (different --id, ports, or network namespaces) to form a swarm. The simulator module can drive mobility and positions.

HTTP API (port --tcp-port, default 8080)
	•	POST /sensor – inject a local fire reading (for tests/integration).
	•	POST /delta – ingress for Delta-CRDT messages (gossip & anti-entropy).
	•	GET  /state – current CRDT view and simple counters.
	•	GET  /stats – health/telemetry: neighbors, timers, caches, counters.
	•	POST /position – set the drone’s (x,y) inside the grid (simulator hook).

Example:

```
curl -X GET http://localhost:8080/stats
```

Configuration knobs (high level)
	•	Networking: --bind, --udp-port, --tcp-port
	•	Discovery: --hello-interval, --hello-jitter, --neighbor-timeout
	•	Gossip: --delta-interval, --fanout, --ttl
	•	Anti-entropy: --anti-entropy (seconds; use 0 or negative to disable)
	•	Sensor/sim: --sample-interval, --confidence-threshold, --grid-size, --id

## Notes
	•	The CRDT layer is payload-agnostic: swap in other CRDTs (counters, maps, LWW registers) without changing dissemination.
	•	UDP HELLOs are minimal (ID only) by design to reduce coupling; data rides on TCP/HTTP.

## License

MIT (or project’s chosen license).
