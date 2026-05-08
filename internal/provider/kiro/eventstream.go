package kiro

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
)

// AWS Event Stream binary protocol implementation
// Reference: https://docs.aws.amazon.com/transcribe/latest/dg/event-stream.html

// EventStreamMessage represents a single message in the AWS Event Stream protocol
type EventStreamMessage struct {
	Headers map[string]string
	Payload []byte
}

// DecodeEventStream reads all messages from an AWS Event Stream binary response
func DecodeEventStream(reader io.Reader) ([]EventStreamMessage, error) {
	var messages []EventStreamMessage

	for {
		msg, err := readOneMessage(reader)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			// Try to continue on non-fatal errors
			break
		}
		messages = append(messages, *msg)
	}

	return messages, nil
}

func readOneMessage(reader io.Reader) (*EventStreamMessage, error) {
	// Read prelude: 4 bytes total length + 4 bytes headers length
	prelude := make([]byte, 8)
	if _, err := io.ReadFull(reader, prelude); err != nil {
		return nil, err
	}

	totalLength := binary.BigEndian.Uint32(prelude[0:4])
	headersLength := binary.BigEndian.Uint32(prelude[4:8])

	// Read prelude CRC (4 bytes)
	preludeCRC := make([]byte, 4)
	if _, err := io.ReadFull(reader, preludeCRC); err != nil {
		return nil, err
	}

	// Validate prelude CRC
	expectedCRC := crc32.ChecksumIEEE(prelude)
	actualCRC := binary.BigEndian.Uint32(preludeCRC)
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("prelude CRC mismatch: expected %d, got %d", expectedCRC, actualCRC)
	}

	// Read headers
	headersBytes := make([]byte, headersLength)
	if headersLength > 0 {
		if _, err := io.ReadFull(reader, headersBytes); err != nil {
			return nil, err
		}
	}

	headers := parseHeaders(headersBytes)

	// Calculate payload length
	// totalLength = prelude(8) + preludeCRC(4) + headers(headersLength) + payload(?) + messageCRC(4)
	payloadLength := int(totalLength) - 8 - 4 - int(headersLength) - 4
	if payloadLength < 0 {
		payloadLength = 0
	}

	// Read payload
	payload := make([]byte, payloadLength)
	if payloadLength > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
	}

	// Read message CRC (4 bytes)
	messageCRC := make([]byte, 4)
	if _, err := io.ReadFull(reader, messageCRC); err != nil {
		return nil, err
	}

	return &EventStreamMessage{
		Headers: headers,
		Payload: payload,
	}, nil
}

func parseHeaders(data []byte) map[string]string {
	headers := make(map[string]string)
	reader := bytes.NewReader(data)

	for reader.Len() > 0 {
		// Read header name length (1 byte)
		nameLen, err := reader.ReadByte()
		if err != nil {
			break
		}

		// Read header name
		name := make([]byte, nameLen)
		if _, err := io.ReadFull(reader, name); err != nil {
			break
		}

		// Read header value type (1 byte)
		valueType, err := reader.ReadByte()
		if err != nil {
			break
		}

		switch valueType {
		case 7: // String type
			// Read value length (2 bytes, big-endian)
			var valueLen uint16
			if err := binary.Read(reader, binary.BigEndian, &valueLen); err != nil {
				break
			}
			value := make([]byte, valueLen)
			if _, err := io.ReadFull(reader, value); err != nil {
				break
			}
			headers[string(name)] = string(value)
		default:
			// Skip unknown types - try to read value length and skip
			// For most types, read 2-byte length then skip
			var valueLen uint16
			if err := binary.Read(reader, binary.BigEndian, &valueLen); err != nil {
				break
			}
			skip := make([]byte, valueLen)
			io.ReadFull(reader, skip)
		}
	}

	return headers
}

// EncodeEventStreamMessage encodes a single message in AWS Event Stream format
func EncodeEventStreamMessage(headers map[string]string, payload []byte) []byte {
	// Encode headers
	var headersBuf bytes.Buffer
	for name, value := range headers {
		headersBuf.WriteByte(byte(len(name)))
		headersBuf.WriteString(name)
		headersBuf.WriteByte(7) // String type
		binary.Write(&headersBuf, binary.BigEndian, uint16(len(value)))
		headersBuf.WriteString(value)
	}

	headersBytes := headersBuf.Bytes()
	headersLength := uint32(len(headersBytes))
	payloadLength := uint32(len(payload))

	// Total length = prelude(8) + preludeCRC(4) + headers + payload + messageCRC(4)
	totalLength := uint32(8 + 4 + headersLength + payloadLength + 4)

	// Build prelude
	prelude := make([]byte, 8)
	binary.BigEndian.PutUint32(prelude[0:4], totalLength)
	binary.BigEndian.PutUint32(prelude[4:8], headersLength)

	// Calculate prelude CRC
	preludeCRC := crc32.ChecksumIEEE(prelude)

	// Build full message (without message CRC)
	var msg bytes.Buffer
	msg.Write(prelude)
	binary.Write(&msg, binary.BigEndian, preludeCRC)
	msg.Write(headersBytes)
	msg.Write(payload)

	// Calculate message CRC
	messageCRC := crc32.ChecksumIEEE(msg.Bytes())
	binary.Write(&msg, binary.BigEndian, messageCRC)

	return msg.Bytes()
}

// ParseKiroEvents extracts assistant response content from event stream messages
func ParseKiroEvents(messages []EventStreamMessage) (content string, inputTokens int, outputTokens int, conversationID string) {
	var contentBuilder bytes.Buffer

	for _, msg := range messages {
		eventType := msg.Headers[":event-type"]
		messageType := msg.Headers[":message-type"]

		// Skip exception messages
		if messageType == "exception" {
			continue
		}

		if eventType == "assistantResponseEvent" && len(msg.Payload) > 0 {
			var event struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err == nil {
				contentBuilder.WriteString(event.Content)
			}
		}

		if eventType == "messageMetadataEvent" && len(msg.Payload) > 0 {
			var event struct {
				ConversationID string `json:"conversationId"`
				Usage          struct {
					InputTokens  int `json:"inputTokens"`
					OutputTokens int `json:"outputTokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(msg.Payload, &event); err == nil {
				conversationID = event.ConversationID
				inputTokens = event.Usage.InputTokens
				outputTokens = event.Usage.OutputTokens
			}
		}
	}

	return contentBuilder.String(), inputTokens, outputTokens, conversationID
}
