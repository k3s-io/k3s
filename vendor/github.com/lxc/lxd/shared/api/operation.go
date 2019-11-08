package api

import (
	"time"
)

// Operation represents a LXD background operation
type Operation struct {
	ID          string                 `json:"id" yaml:"id"`
	Class       string                 `json:"class" yaml:"class"`
	Description string                 `json:"description" yaml:"description"`
	CreatedAt   time.Time              `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" yaml:"updated_at"`
	Status      string                 `json:"status" yaml:"status"`
	StatusCode  StatusCode             `json:"status_code" yaml:"status_code"`
	Resources   map[string][]string    `json:"resources" yaml:"resources"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"metadata"`
	MayCancel   bool                   `json:"may_cancel" yaml:"may_cancel"`
	Err         string                 `json:"err" yaml:"err"`

	// API extension: operation_location
	Location string `json:"location" yaml:"location"`
}
