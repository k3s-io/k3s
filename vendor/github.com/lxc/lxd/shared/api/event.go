package api

import (
	"encoding/json"
	"time"
)

// Event represents an event entry (over websocket)
type Event struct {
	Type      string          `yaml:"type" json:"type"`
	Timestamp time.Time       `yaml:"timestamp" json:"timestamp"`
	Metadata  json.RawMessage `yaml:"metadata" json:"metadata"`

	// API extension: event_location
	Location string `yaml:"location,omitempty" json:"location,omitempty"`
}

// EventLogging represents a logging type event entry (admin only)
type EventLogging struct {
	Message string            `yaml:"message" json:"message"`
	Level   string            `yaml:"level" json:"level"`
	Context map[string]string `yaml:"context" json:"context"`
}

// EventLifecycle represets a lifecycle type event entry
//
// API extension: event_lifecycle
type EventLifecycle struct {
	Action  string                 `yaml:"action" json:"action"`
	Source  string                 `yaml:"source" json:"source"`
	Context map[string]interface{} `yaml:"context,omitempty" json:"context,omitempty"`
}
