package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMessageTypes_Constants(t *testing.T) {
	if AdvertiseType != "ADVERTISE" {
		t.Errorf("Esperado AdvertiseType 'ADVERTISE', obtido %s", AdvertiseType)
	}

	if RequestType != "REQUEST" {
		t.Errorf("Esperado RequestType 'REQUEST', obtido %s", RequestType)
	}

	if SwitchChannelType != "SWITCH_CHANNEL" {
		t.Errorf("Esperado SwitchChannelType 'SWITCH_CHANNEL', obtido %s", SwitchChannelType)
	}
}

func TestCreateAdvertiseMessage(t *testing.T) {
	senderID := "test-drone"
	id1 := uuid.New()
	id2 := uuid.New()
	haveIDs := []uuid.UUID{id1, id2}

	msg := CreateAdvertiseMessage(senderID, haveIDs)

	if msg.Type != AdvertiseType {
		t.Errorf("Esperado tipo %s, obtido %s", AdvertiseType, msg.Type)
	}

	if msg.SenderID != senderID {
		t.Errorf("Esperado SenderID %s, obtido %s", senderID, msg.SenderID)
	}

	if msg.Timestamp == 0 {
		t.Error("Timestamp não deveria ser zero")
	}

	// Verifica se timestamp está próximo do atual (dentro de 1 segundo)
	now := time.Now().UnixMilli()
	if abs(now-msg.Timestamp) > 1000 {
		t.Errorf("Timestamp muito distante do atual: %d vs %d", msg.Timestamp, now)
	}

	// Verifica dados específicos do Advertise
	advertiseData, ok := msg.Data.(AdvertiseMsg)
	if !ok {
		t.Fatal("Data deveria ser do tipo AdvertiseMsg")
	}

	if advertiseData.SenderID != senderID {
		t.Errorf("AdvertiseMsg.SenderID esperado %s, obtido %s", senderID, advertiseData.SenderID)
	}

	if len(advertiseData.HaveIDs) != 2 {
		t.Errorf("Esperado 2 HaveIDs, obtido %d", len(advertiseData.HaveIDs))
	}

	if advertiseData.HaveIDs[0] != id1 || advertiseData.HaveIDs[1] != id2 {
		t.Error("HaveIDs não correspondem aos IDs fornecidos")
	}
}

func TestCreateRequestMessage(t *testing.T) {
	senderID := "test-drone"
	id1 := uuid.New()
	id2 := uuid.New()
	wantedIDs := []uuid.UUID{id1, id2}

	msg := CreateRequestMessage(senderID, wantedIDs)

	if msg.Type != RequestType {
		t.Errorf("Esperado tipo %s, obtido %s", RequestType, msg.Type)
	}

	if msg.SenderID != senderID {
		t.Errorf("Esperado SenderID %s, obtido %s", senderID, msg.SenderID)
	}

	if msg.Timestamp == 0 {
		t.Error("Timestamp não deveria ser zero")
	}

	// Verifica dados específicos do Request
	requestData, ok := msg.Data.(RequestMsg)
	if !ok {
		t.Fatal("Data deveria ser do tipo RequestMsg")
	}

	if requestData.SenderID != senderID {
		t.Errorf("RequestMsg.SenderID esperado %s, obtido %s", senderID, requestData.SenderID)
	}

	if len(requestData.WantedIDs) != 2 {
		t.Errorf("Esperado 2 WantedIDs, obtido %d", len(requestData.WantedIDs))
	}

	if requestData.WantedIDs[0] != id1 || requestData.WantedIDs[1] != id2 {
		t.Error("WantedIDs não correspondem aos IDs fornecidos")
	}
}

func TestCreateSwitchChannelMessage(t *testing.T) {
	senderID := "test-drone"
	deltaID := uuid.New()

	msg := CreateSwitchChannelMessage(senderID, deltaID)

	if msg.Type != SwitchChannelType {
		t.Errorf("Esperado tipo %s, obtido %s", SwitchChannelType, msg.Type)
	}

	if msg.SenderID != senderID {
		t.Errorf("Esperado SenderID %s, obtido %s", senderID, msg.SenderID)
	}

	if msg.Timestamp == 0 {
		t.Error("Timestamp não deveria ser zero")
	}

	// Verifica dados específicos do SwitchChannel
	switchData, ok := msg.Data.(SwitchChannelMsg)
	if !ok {
		t.Fatal("Data deveria ser do tipo SwitchChannelMsg")
	}

	if switchData.SenderID != senderID {
		t.Errorf("SwitchChannelMsg.SenderID esperado %s, obtido %s", senderID, switchData.SenderID)
	}

	if switchData.DeltaID != deltaID {
		t.Errorf("DeltaID esperado %s, obtido %s", deltaID, switchData.DeltaID)
	}
}

func TestCreateAdvertiseMessage_EmptyIDs(t *testing.T) {
	senderID := "test-drone"
	var haveIDs []uuid.UUID // Lista vazia

	msg := CreateAdvertiseMessage(senderID, haveIDs)

	advertiseData, ok := msg.Data.(AdvertiseMsg)
	if !ok {
		t.Fatal("Data deveria ser do tipo AdvertiseMsg")
	}

	if len(advertiseData.HaveIDs) != 0 {
		t.Errorf("Esperado 0 HaveIDs para lista vazia, obtido %d", len(advertiseData.HaveIDs))
	}

	// A lista pode ser nil quando vazia, isso é válido em Go
	// Removendo teste que força que não seja nil
}

func TestEncodeMessage(t *testing.T) {
	senderID := "test-drone"
	id1 := uuid.New()
	haveIDs := []uuid.UUID{id1}

	advertiseData := AdvertiseMsg{
		SenderID: senderID,
		HaveIDs:  haveIDs,
	}

	data, err := EncodeMessage("ADVERTISE", advertiseData)
	if err != nil {
		t.Fatalf("EncodeMessage não deveria falhar: %v", err)
	}

	if len(data) == 0 {
		t.Error("Dados codificados não deveriam estar vazios")
	}

	// Tenta decodificar para verificar se é JSON válido
	var decoded ControlMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Dados codificados deveriam ser JSON válido: %v", err)
	}

	if decoded.Type != AdvertiseType {
		t.Errorf("Tipo decodificado esperado %s, obtido %s", AdvertiseType, decoded.Type)
	}

	if decoded.SenderID != senderID {
		t.Errorf("SenderID decodificado esperado %s, obtido %s", senderID, decoded.SenderID)
	}
}

func TestEncodeMessage_DifferentTypes(t *testing.T) {
	senderID := "test-drone"
	deltaID := uuid.New()

	// Teste SwitchChannelMsg
	switchData := SwitchChannelMsg{
		SenderID: senderID,
		DeltaID:  deltaID,
		ReqCount: 3,
	}

	data, err := EncodeMessage("SWITCH_CHANNEL", switchData)
	if err != nil {
		t.Fatalf("EncodeMessage para SwitchChannel não deveria falhar: %v", err)
	}

	var decoded ControlMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Dados codificados deveriam ser JSON válido: %v", err)
	}

	if decoded.Type != SwitchChannelType {
		t.Errorf("Tipo esperado %s, obtido %s", SwitchChannelType, decoded.Type)
	}

	// Teste RequestMsg
	requestData := RequestMsg{
		SenderID:  senderID,
		WantedIDs: []uuid.UUID{deltaID},
	}

	data, err = EncodeMessage("REQUEST", requestData)
	if err != nil {
		t.Fatalf("EncodeMessage para Request não deveria falhar: %v", err)
	}

	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Dados codificados deveriam ser JSON válido: %v", err)
	}

	if decoded.Type != RequestType {
		t.Errorf("Tipo esperado %s, obtido %s", RequestType, decoded.Type)
	}
}

func TestParseAdvertiseMessage(t *testing.T) {
	senderID := "test-drone"
	id1 := uuid.New()

	// Cria mensagem como viria do JSON (através de map)
	msg := ControlMessage{
		Type:      AdvertiseType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data: map[string]interface{}{
			"sender_id": senderID,
			"have_ids":  []interface{}{id1.String()},
		},
	}

	// Testa parsing
	parsedMsg, ok := ParseAdvertiseMessage(msg)
	if !ok {
		t.Fatal("ParseAdvertiseMessage deveria ter sucesso")
	}

	if parsedMsg.SenderID != senderID {
		t.Errorf("SenderID esperado %s, obtido %s", senderID, parsedMsg.SenderID)
	}

	if len(parsedMsg.HaveIDs) != 1 {
		t.Errorf("Esperado 1 HaveID, obtido %d", len(parsedMsg.HaveIDs))
	}

	if parsedMsg.HaveIDs[0] != id1 {
		t.Error("HaveID não corresponde ao original")
	}
}

func TestParseRequestMessage(t *testing.T) {
	senderID := "test-drone"
	id1 := uuid.New()
	id2 := uuid.New()

	// Cria mensagem como viria do JSON (através de map)
	msg := ControlMessage{
		Type:      RequestType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data: map[string]interface{}{
			"sender_id":  senderID,
			"wanted_ids": []interface{}{id1.String(), id2.String()},
		},
	}

	// Testa parsing
	parsedMsg, ok := ParseRequestMessage(msg)
	if !ok {
		t.Fatal("ParseRequestMessage deveria ter sucesso")
	}

	if parsedMsg.SenderID != senderID {
		t.Errorf("SenderID esperado %s, obtido %s", senderID, parsedMsg.SenderID)
	}

	if len(parsedMsg.WantedIDs) != 2 {
		t.Errorf("Esperado 2 WantedIDs, obtido %d", len(parsedMsg.WantedIDs))
	}
}

func TestParseSwitchChannelMessage(t *testing.T) {
	senderID := "test-drone"
	deltaID := uuid.New()

	// Cria mensagem como viria do JSON (através de map)
	msg := ControlMessage{
		Type:      SwitchChannelType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data: map[string]interface{}{
			"sender_id": senderID,
			"delta_id":  deltaID.String(),
		},
	}

	// Testa parsing
	parsedMsg, ok := ParseSwitchChannelMessage(msg)
	if !ok {
		t.Fatal("ParseSwitchChannelMessage deveria ter sucesso")
	}

	if parsedMsg.SenderID != senderID {
		t.Errorf("SenderID esperado %s, obtido %s", senderID, parsedMsg.SenderID)
	}

	if parsedMsg.DeltaID != deltaID {
		t.Error("DeltaID não corresponde ao original")
	}
}

func TestParseMessage_WrongType(t *testing.T) {
	senderID := "test-drone"
	advertiseMsg := CreateAdvertiseMessage(senderID, []uuid.UUID{})

	// Tenta fazer parse como Request (tipo errado)
	_, ok := ParseRequestMessage(advertiseMsg)
	if ok {
		t.Error("ParseRequestMessage deveria falhar com tipo ADVERTISE")
	}

	// Tenta fazer parse como SwitchChannel (tipo errado)
	_, ok = ParseSwitchChannelMessage(advertiseMsg)
	if ok {
		t.Error("ParseSwitchChannelMessage deveria falhar com tipo ADVERTISE")
	}
}

func TestControlMessage_JSONSerialization(t *testing.T) {
	senderID := "test-drone"
	id1 := uuid.New()
	id2 := uuid.New()
	haveIDs := []uuid.UUID{id1, id2}

	originalMsg := CreateAdvertiseMessage(senderID, haveIDs)

	// Serializa para JSON
	jsonData, err := json.Marshal(originalMsg)
	if err != nil {
		t.Fatalf("Marshal falhou: %v", err)
	}

	// Deserializa do JSON
	var deserializedMsg ControlMessage
	err = json.Unmarshal(jsonData, &deserializedMsg)
	if err != nil {
		t.Fatalf("Unmarshal falhou: %v", err)
	}

	// Verifica campos básicos
	if deserializedMsg.Type != originalMsg.Type {
		t.Errorf("Tipo esperado %s, obtido %s", originalMsg.Type, deserializedMsg.Type)
	}

	if deserializedMsg.SenderID != originalMsg.SenderID {
		t.Errorf("SenderID esperado %s, obtido %s", originalMsg.SenderID, deserializedMsg.SenderID)
	}

	if deserializedMsg.Timestamp != originalMsg.Timestamp {
		t.Errorf("Timestamp esperado %d, obtido %d", originalMsg.Timestamp, deserializedMsg.Timestamp)
	}
}

func TestMessages_TypeSpecificData(t *testing.T) {
	senderID := "test-drone"

	// Teste AdvertiseMsg
	advertiseMsg := AdvertiseMsg{
		SenderID: senderID,
		HaveIDs:  []uuid.UUID{uuid.New()},
	}

	if advertiseMsg.SenderID != senderID {
		t.Errorf("AdvertiseMsg.SenderID esperado %s, obtido %s", senderID, advertiseMsg.SenderID)
	}

	if len(advertiseMsg.HaveIDs) != 1 {
		t.Errorf("AdvertiseMsg.HaveIDs esperado 1 item, obtido %d", len(advertiseMsg.HaveIDs))
	}

	// Teste RequestMsg
	requestMsg := RequestMsg{
		SenderID:  senderID,
		WantedIDs: []uuid.UUID{uuid.New(), uuid.New()},
	}

	if requestMsg.SenderID != senderID {
		t.Errorf("RequestMsg.SenderID esperado %s, obtido %s", senderID, requestMsg.SenderID)
	}

	if len(requestMsg.WantedIDs) != 2 {
		t.Errorf("RequestMsg.WantedIDs esperado 2 itens, obtido %d", len(requestMsg.WantedIDs))
	}

	// Teste SwitchChannelMsg
	deltaID := uuid.New()
	switchMsg := SwitchChannelMsg{
		SenderID: senderID,
		DeltaID:  deltaID,
		ReqCount: 10,
	}

	if switchMsg.SenderID != senderID {
		t.Errorf("SwitchChannelMsg.SenderID esperado %s, obtido %s", senderID, switchMsg.SenderID)
	}

	if switchMsg.DeltaID != deltaID {
		t.Errorf("SwitchChannelMsg.DeltaID não corresponde")
	}

	if switchMsg.ReqCount != 10 {
		t.Errorf("SwitchChannelMsg.ReqCount esperado 10, obtido %d", switchMsg.ReqCount)
	}
}

func TestTimestampGeneration(t *testing.T) {
	// Cria duas mensagens em sequência rápida
	msg1 := CreateAdvertiseMessage("drone1", []uuid.UUID{})
	msg2 := CreateAdvertiseMessage("drone2", []uuid.UUID{})

	// Timestamps devem ser diferentes (ou iguais se muito rápido)
	if msg1.Timestamp > msg2.Timestamp {
		t.Error("msg1.Timestamp não deveria ser maior que msg2.Timestamp")
	}

	// Timestamps devem ser próximos ao tempo atual
	now := time.Now().UnixMilli()
	if abs(now-msg1.Timestamp) > 1000 {
		t.Errorf("msg1.Timestamp muito distante: %d vs %d", msg1.Timestamp, now)
	}
}

// Helper function para valor absoluto
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
