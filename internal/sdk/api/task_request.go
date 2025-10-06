package api

import (
	"os"
	"path/filepath"
	"time"
)

// VerificationToken is a nonsense Terraform Cloud API token that should NEVER be valid.
const verificationToken = "test-token"

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
