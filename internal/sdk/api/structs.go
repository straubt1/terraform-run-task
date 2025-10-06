// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// VerificationToken is a nonsense Terraform Cloud API token that should NEVER be valid.
const verificationToken = "test-token"
const JsonApiMediaTypeHeader = "application/vnd.api+json"

type TaskStatus string

const (
	TaskFailed  TaskStatus = "failed"
	TaskPassed  TaskStatus = "passed"
	TaskRunning TaskStatus = "running"
)

type TaskStage string

const (
	PrePlan   TaskStage = "pre_plan"
	PostPlan  TaskStage = "post_plan"
	PreApply  TaskStage = "pre_apply"
	PostApply TaskStage = "post_apply"
)

// TaskRequest is the top level message sent to the Run Task.
type TaskRequest struct {
	AccessToken                     string    `json:"access_token"`
	ConfigurationVersionDownloadURL string    `json:"configuration_version_download_url,omitempty"`
	ConfigurationVersionID          string    `json:"configuration_version_id,omitempty"`
	IsSpeculative                   bool      `json:"is_speculative"`
	OrganizationName                string    `json:"organization_name"`
	PayloadVersion                  int       `json:"payload_version"`
	RunAppURL                       string    `json:"run_app_url"`
	RunCreatedAt                    time.Time `json:"run_created_at"`
	RunCreatedBy                    string    `json:"run_created_by"`
	RunID                           string    `json:"run_id"`
	RunMessage                      string    `json:"run_message"`
	Stage                           TaskStage `json:"stage"`
	TaskResultCallbackURL           string    `json:"task_result_callback_url"`
	TaskResultEnforcementLevel      string    `json:"task_result_enforcement_level"`
	TaskResultID                    string    `json:"task_result_id"`
	VcsBranch                       string    `json:"vcs_branch,omitempty"`
	VcsCommitURL                    string    `json:"vcs_commit_url,omitempty"`
	VcsPullRequestURL               string    `json:"vcs_pull_request_url,omitempty"`
	VcsRepoURL                      string    `json:"vcs_repo_url,omitempty"`
	WorkspaceAppURL                 string    `json:"workspace_app_url"`
	WorkspaceID                     string    `json:"workspace_id"`
	WorkspaceName                   string    `json:"workspace_name"`
	WorkspaceWorkingDirectory       string    `json:"workspace_working_directory,omitempty"`
	PlanJSONAPIURL                  string    `json:"plan_json_api_url,omitempty"`

	// Internal use only, not part of the API, nor saved to disk after parsing to JSON
	TaskDirectory string `json:"-"` // Directory where the run task is executed
}

// IsEndpointValidation returns true if the Request is from the
// run task service to validate this API endpoint. Callers should
// immediately return an HTTP 200 status code for these requests.
func (r TaskRequest) IsEndpointValidation() bool {
	return r.AccessToken == verificationToken
}

// During at Task execution for a specific stage, create the directory structure
// and save the directory to the TaskRequest struct for easy access later.
func (r *TaskRequest) CreateRunTaskDirectoryStructure() (string, error) {
	// Prefix the stage folder with a number to make it easier to read
	var stageFolder string
	stageString := string(r.Stage)
	switch r.Stage {
	case PrePlan:
		stageFolder = "1_" + stageString
	case PostPlan:
		stageFolder = "2_" + stageString
	case PreApply:
		stageFolder = "3_" + stageString
	case PostApply:
		stageFolder = "4_" + stageString
	default:
		stageFolder = stageString
	}
	path := filepath.Join(".", r.WorkspaceName, r.RunID, stageFolder)
	r.TaskDirectory = path
	err := os.MkdirAll(path, os.ModePerm)
	return path, err
}

// TaskResponse is the top level message sent back to HCP Terraform.
type TaskResponse struct {
	Data ResponseData `json:"data"`
}

// func NewTaskResponse(status TaskStatus, message string) *TaskResponse {
// Create a new TaskResponse with empty values
// These will get set later
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

// ntr.AddOutcome("save-request", "Request saved to file successfully", "label-text", api.TagLevelInfo)
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
	// A short message describing the status of the task.
	Message string `json:"message,omitempty"`
	// Must be one of TaskFailed, TaskPassed or TaskRunning
	Status TaskStatus `json:"status"`
	// URL that the user can use to get more information from the external service
	URL string `json:"url,omitempty"`
}
