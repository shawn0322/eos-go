package eos

import (
	"encoding/binary"
	"errors"
	"io"

	"fmt"

	"reflect"
)

// Work-in-progress p2p comms implementation
//
// See /home/abourget/build/eos3/plugins/net_plugin/include/eosio/net_plugin/protocol.hpp:219
//

type P2PMessageType byte

const (
	HandshakeMessageType P2PMessageType = iota
	GoAwayMessageType
	TimeMessageType
	NoticeMessageType
	RequestMessageType
	SyncRequestMessageType
	SignedBlockSummaryMessageType
	SignedBlockMessageType
	SignedTransactionMessageType
	PackedTransactionMessageType
)

type MessageAttributes struct {
	Name        string
	ReflectType reflect.Type
}

var messageAttributes = []MessageAttributes{
	{Name: "Handshake", ReflectType: nil},
	{Name: "GoAway", ReflectType: nil},
	{Name: "Time", ReflectType: reflect.TypeOf(TimeMessage{})},
	{Name: "Notice", ReflectType: nil},
	{Name: "Request", ReflectType: nil},
	{Name: "SyncRequest", ReflectType: nil},
	{Name: "SignedBlockSummary", ReflectType: nil},
	{Name: "SignedBlock", ReflectType: nil},
	{Name: "SignedTransaction", ReflectType: nil},
	{Name: "PackedTransaction", ReflectType: nil},
}

var UnknownMessageTypeError = errors.New("unknown type")

func NewMessageType(aType byte) (t P2PMessageType, err error) {

	t = P2PMessageType(aType)
	if !t.isValid() {
		err = UnknownMessageTypeError
		return
	}

	return
}

func (t P2PMessageType) isValid() bool {

	index := byte(t)
	return int(index) < len(messageAttributes) && index >= 0

}

func (t P2PMessageType) Name() (string, bool) {

	index := byte(t)

	if !t.isValid() {
		return "Unknown", false
	}

	attr := messageAttributes[index]
	return attr.Name, true
}

func (t P2PMessageType) Attributes() (MessageAttributes, bool) {

	index := byte(t)

	if !t.isValid() {
		return MessageAttributes{}, false
	}

	attr := messageAttributes[index]
	return attr, true
}

type P2PMessage struct {
	Length  uint32
	Type    P2PMessageType
	Payload []byte
}

func (p2pMsg P2PMessage) AsMessage() (interface{}, error) {

	attr, ok := p2pMsg.Type.Attributes()

	if !ok {
		return nil, UnknownMessageTypeError
	}

	if attr.ReflectType == nil {
		return nil, errors.New("Missing reflect type ")
	}

	msg := reflect.New(attr.ReflectType)

	err := p2pMsg.DecodePayload(msg.Interface())
	if err != nil {
		return nil, err
	}

	return msg.Interface(), err
}

func (p2pMsg P2PMessage) DecodePayload(message interface{}) error {

	attr, ok := p2pMsg.Type.Attributes()

	if !ok {
		return UnknownMessageTypeError
	}

	if attr.ReflectType == nil {
		return errors.New("Missing reflect type ")
	}

	messageType := reflect.TypeOf(message).Elem()
	if messageType != attr.ReflectType {
		return errors.New(fmt.Sprintf("Given message type [%s] to not match payload type [%s]", messageType.Name(), attr.ReflectType.Name()))
	}

	return UnmarshalBinary(p2pMsg.Payload, message)

}

func (p2pMsg P2PMessage) MarshalBinary() ([]byte, error) {

	data := make([]byte, p2pMsg.Length+4, p2pMsg.Length+4)
	binary.LittleEndian.PutUint32(data[0:4], p2pMsg.Length)
	data[4] = byte(p2pMsg.Type)
	copy(data[5:], p2pMsg.Payload)

	return data, nil
}

func (p2pMsg *P2PMessage) UnmarshalBinaryRead(r io.Reader) (err error) {

	lengthBytes := make([]byte, 4, 4)
	_, err = r.Read(lengthBytes)
	if err != nil {
		fmt.Errorf("error: [%s]\n", err)
		return
	}

	size := binary.LittleEndian.Uint32(lengthBytes)

	payloadBytes := make([]byte, size, size)

	_, err = io.ReadFull(r, payloadBytes)

	if err != nil {
		return
	}
	//fmt.Printf("--> Payload length [%d] read count [%d]\n", size, count)

	if size < 1 {
		return errors.New("empty message")
	}

	//headerBytes := append(lengthBytes, payloadBytes[:int(math.Min(float64(10), float64(len(payloadBytes))))]...)

	//fmt.Printf("Length: [%s] Payload: [%s]\n", hex.EncodeToString(lengthBytes), hex.EncodeToString(payloadBytes[:int(math.Min(float64(1000), float64(len(payloadBytes))))]))

	messageType, err := NewMessageType(payloadBytes[0])
	if err != nil {
		return
	}

	*p2pMsg = P2PMessage{
		Length:  size,
		Type:    messageType,
		Payload: payloadBytes[1:],
	}

	return nil
}
