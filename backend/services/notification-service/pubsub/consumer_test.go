package pubsub

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// MockBroadcaster implements MessageBroadcaster for testing
type MockBroadcaster struct {
	messages map[string][][]byte
}

func NewMockBroadcaster() *MockBroadcaster {
	return &MockBroadcaster{
		messages: make(map[string][][]byte),
	}
}

func (m *MockBroadcaster) Broadcast(channel string, message []byte) {
	m.messages[channel] = append(m.messages[channel], message)
}

func (m *MockBroadcaster) GetMessages(channel string) [][]byte {
	return m.messages[channel]
}

func (m *MockBroadcaster) GetAllChannels() []string {
	channels := make([]string, 0, len(m.messages))
	for ch := range m.messages {
		channels = append(channels, ch)
	}
	return channels
}

func TestGetMessageType(t *testing.T) {
	tests := []struct {
		channel  string
		expected string
	}{
		{"build_logs:proj-123:build-456", "build_log"},
		{"build_logs:proj-123", "build_log"},
		{"deployment_logs:proj-123", "deployment_log"},
		{"events:build_complete", "event"},
		{"other_channel", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			result := getMessageType(tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractProjectID(t *testing.T) {
	tests := []struct {
		channel  string
		expected string
	}{
		{"build_logs:proj-123:build-456", "proj-123"},
		{"deployment_logs:proj-abc", "proj-abc"},
		{"events:some-event", "some-event"},
		{"single", ""},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			result := extractProjectID(tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractBuildID(t *testing.T) {
	tests := []struct {
		channel  string
		expected string
	}{
		{"build_logs:proj-123:build-456", "build-456"},
		{"build_logs:proj-123", ""},
		{"deployment_logs:proj-abc", ""},
		{"single", ""},
	}

	for _, tt := range tests {
		t.Run(tt.channel, func(t *testing.T) {
			result := extractBuildID(tt.channel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewConsumer(t *testing.T) {
	log := zerolog.New(os.Stdout)
	broadcaster := NewMockBroadcaster()

	consumer := NewConsumer(nil, broadcaster, log)

	assert.NotNil(t, consumer)
	assert.Equal(t, broadcaster, consumer.broadcaster)
	assert.Len(t, consumer.channels, 3)
	assert.Contains(t, consumer.channels, "build_logs:*")
	assert.Contains(t, consumer.channels, "deployment_logs:*")
	assert.Contains(t, consumer.channels, "events:*")
}

func TestLogMessageSerialization(t *testing.T) {
	msg := LogMessage{
		Type:      "build_log",
		ProjectID: "proj-123",
		BuildID:   "build-456",
		Timestamp: "2024-01-15T10:30:00Z",
		Message:   "Building project...",
		Level:     "info",
	}

	assert.Equal(t, "build_log", msg.Type)
	assert.Equal(t, "proj-123", msg.ProjectID)
	assert.Equal(t, "build-456", msg.BuildID)
	assert.Equal(t, "info", msg.Level)
}

