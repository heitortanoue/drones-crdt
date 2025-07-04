package crdt

type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type FireMeta struct {
	Timestamp  int64   `json:"timestamp"`
	Confidence float64 `json:"confidence"`
}

type FireDeltaEntry struct {
	Dot  Dot      `json:"dot"` // (drone_id + counter)
	Cell Cell     `json:"cell"`
	Meta FireMeta `json:"meta"`
}

type FireDelta struct {
	Context DotContext       `json:"context"` // o clock + dot_cloud enxuto
	Entries []FireDeltaEntry `json:"entries"` // só as ops novas
}