package protocol

type HelloMessage struct {
    ID        string `json:"id"`
    Version   int    `json:"version"`
}
