// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"strings"
)

const JsonApiMediaTypeHeader = "application/vnd.api+json"

// TaskResponse is the top level message sent back to HCP Terraform.
type TaskResponse struct {
	Data ResponseData `json:"data"`
}

// Create a new TaskResponse with empty values
// These will get set later once the Task is done working
func NewTaskResponse() *TaskResponse {
	return &TaskResponse{
		Data: ResponseData{
			Type:       "task-results",
			Attributes: ResponseAttributes{},
			Relationships: &ResponseRelationships{
				Outcomes: ResponseOutcomes{
					Data: []ResponseOutcome{},
				},
			},
		},
	}
}

// Add an outcome to the TaskResponse
// Think of this like a step of the Run Task
// You can have any number of outcomes (including zero)
func (r *TaskResponse) AddOutcome(outcomeId string, description string, body string, url string, label string, level ResponseTagLevel) *TaskResponse {
	outcome := ResponseOutcome{
		Type: "task-result-outcomes",
		Attributes: ResponseOutcomeAttributes{
			OutcomeID:   outcomeId,
			Description: description,
			Body:        body,
			URL:         url,
			Tags: Tags{
				Status: []Tag{
					{
						Label: label,
						Level: level,
					},
				},
			},
		},
	}
	r.Data.Relationships.Outcomes.Data = append(r.Data.Relationships.Outcomes.Data, outcome)
	return r
}

// Set the overall result of the TaskResponse
// This should be called after adding all outcomes
func (r *TaskResponse) SetResult(status TaskStatus, message string) *TaskResponse {
	r.Data.Attributes.Status = status
	r.Data.Attributes.Message = message
	return r
}

// Optionally set a URL for the TaskResponse
func (r *TaskResponse) WithUrl(url string) *TaskResponse {
	// Basic validation to ensure the URL starts with http:// or https://
	// Else don't set it - this will break the UI if not set correctly
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		r.Data.Attributes.URL = url
	}
	return r
}

// IsPassed checks if the TaskResponse has any outcomes with error tags
func (r *TaskResponse) IsPassed() bool {
	for _, outcome := range r.Data.Relationships.Outcomes.Data {
		for _, tag := range outcome.Attributes.Tags.Status {
			if tag.Level == TagLevelError {
				return false
			}
		}
	}
	return true // If no error tags found, return true
}

type ResponseData struct {
	Type          string                 `json:"type"`
	Attributes    ResponseAttributes     `json:"attributes"`
	Relationships *ResponseRelationships `json:"relationships,omitempty"`
}

type ResponseRelationships struct {
	Outcomes ResponseOutcomes `json:"outcomes,omitempty"`
}

type ResponseOutcomes struct {
	Data []ResponseOutcome `json:"data"`
}

type ResponseOutcome struct {
	Type       string                    `json:"type"`
	Attributes ResponseOutcomeAttributes `json:"attributes"`
}

type ResponseOutcomeAttributes struct {
	OutcomeID   string `json:"outcome-id"`
	Description string `json:"description,omitempty"`
	Tags        Tags   `json:"tags,omitempty"`
	Body        string `json:"body,omitempty"`
	URL         string `json:"url,omitempty"`
}

// You can add additional tags here if needed
// KIS here and just add to the Status tags for now
type Tags struct {
	Status []Tag `json:"status,omitempty"`
	// Severity []Tag `json:"severity,omitempty"`
	// Custom   []Tag `json:"custom,omitempty"`
}

type ResponseTagLevel string

const (
	TagLevelNone    ResponseTagLevel = "none"
	TagLevelInfo    ResponseTagLevel = "info"
	TagLevelWarning ResponseTagLevel = "warning"
	TagLevelError   ResponseTagLevel = "error"
)

type Tag struct {
	Label string           `json:"label"`
	Level ResponseTagLevel `json:"level"` // none, info, warning, error
}

type ResponseAttributes struct {
	Message string     `json:"message,omitempty"`
	Status  TaskStatus `json:"status"`
	URL     string     `json:"url,omitempty"`
}
