package protocol

import (
	"time"

	"github.com/google/uuid"
)

// MessageType define os tipos de mensagens de controle
type MessageType string

const (
	AdvertiseType     MessageType = "ADVERTISE"
	RequestType       MessageType = "REQUEST"
	SwitchChannelType MessageType = "SWITCH_CHANNEL"
)

// ControlMessage representa uma mensagem genérica de controle
type ControlMessage struct {
	Type      MessageType `json:"type"`
	SenderID  string      `json:"sender_id"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// AdvertiseMsg anuncia deltas disponíveis (Requisito F3)
type AdvertiseMsg struct {
	SenderID string      `json:"sender_id"`
	HaveIDs  []uuid.UUID `json:"have_ids"`
}

// RequestMsg solicita deltas específicos (Requisito F3)
type RequestMsg struct {
	SenderID  string      `json:"sender_id"`
	WantedIDs []uuid.UUID `json:"wanted_ids"`
}

// SwitchChannelMsg coordena transmissão de delta (Requisito F3)
type SwitchChannelMsg struct {
	SenderID string    `json:"sender_id"`
	DeltaID  uuid.UUID `json:"delta_id"`
}

// CreateAdvertiseMessage cria uma mensagem Advertise
func CreateAdvertiseMessage(senderID string, haveIDs []uuid.UUID) ControlMessage {
	return ControlMessage{
		Type:      AdvertiseType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data: AdvertiseMsg{
			SenderID: senderID,
			HaveIDs:  haveIDs,
		},
	}
}

// CreateRequestMessage cria uma mensagem Request
func CreateRequestMessage(senderID string, wantedIDs []uuid.UUID) ControlMessage {
	return ControlMessage{
		Type:      RequestType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data: RequestMsg{
			SenderID:  senderID,
			WantedIDs: wantedIDs,
		},
	}
}

// CreateSwitchChannelMessage cria uma mensagem SwitchChannel
func CreateSwitchChannelMessage(senderID string, deltaID uuid.UUID) ControlMessage {
	return ControlMessage{
		Type:      SwitchChannelType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data: SwitchChannelMsg{
			SenderID: senderID,
			DeltaID:  deltaID,
		},
	}
}

// ParseAdvertiseMessage extrai dados de uma mensagem Advertise
func ParseAdvertiseMessage(msg ControlMessage) (*AdvertiseMsg, bool) {
	if msg.Type != AdvertiseType {
		return nil, false
	}

	// Converte interface{} para map e depois para struct
	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		advertise := &AdvertiseMsg{
			SenderID: msg.SenderID,
		}

		// Converte HaveIDs
		if haveIDsInterface, exists := dataMap["have_ids"]; exists {
			if haveIDsSlice, ok := haveIDsInterface.([]interface{}); ok {
				advertise.HaveIDs = make([]uuid.UUID, 0, len(haveIDsSlice))
				for _, idInterface := range haveIDsSlice {
					if idStr, ok := idInterface.(string); ok {
						if id, err := uuid.Parse(idStr); err == nil {
							advertise.HaveIDs = append(advertise.HaveIDs, id)
						}
					}
				}
			}
		}

		return advertise, true
	}

	return nil, false
}

// ParseRequestMessage extrai dados de uma mensagem Request
func ParseRequestMessage(msg ControlMessage) (*RequestMsg, bool) {
	if msg.Type != RequestType {
		return nil, false
	}

	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		request := &RequestMsg{
			SenderID: msg.SenderID,
		}

		// Converte WantedIDs
		if wantedIDsInterface, exists := dataMap["wanted_ids"]; exists {
			if wantedIDsSlice, ok := wantedIDsInterface.([]interface{}); ok {
				request.WantedIDs = make([]uuid.UUID, 0, len(wantedIDsSlice))
				for _, idInterface := range wantedIDsSlice {
					if idStr, ok := idInterface.(string); ok {
						if id, err := uuid.Parse(idStr); err == nil {
							request.WantedIDs = append(request.WantedIDs, id)
						}
					}
				}
			}
		}

		return request, true
	}

	return nil, false
}

// ParseSwitchChannelMessage extrai dados de uma mensagem SwitchChannel
func ParseSwitchChannelMessage(msg ControlMessage) (*SwitchChannelMsg, bool) {
	if msg.Type != SwitchChannelType {
		return nil, false
	}

	if dataMap, ok := msg.Data.(map[string]interface{}); ok {
		switchMsg := &SwitchChannelMsg{
			SenderID: msg.SenderID,
		}

		// Converte DeltaID
		if deltaIDInterface, exists := dataMap["delta_id"]; exists {
			if deltaIDStr, ok := deltaIDInterface.(string); ok {
				if id, err := uuid.Parse(deltaIDStr); err == nil {
					switchMsg.DeltaID = id
				}
			}
		}

		return switchMsg, true
	}

	return nil, false
}

// getCurrentTimestamp retorna timestamp atual em milissegundos
func getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}
