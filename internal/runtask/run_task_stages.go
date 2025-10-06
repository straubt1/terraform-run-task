// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package runtask

import (
	"fmt"
	"log"
	"os"

	"github.com/straubt1/terraform-run-task/internal/helper"
	"github.com/straubt1/terraform-run-task/internal/sdk/api"
	"github.com/straubt1/terraform-run-task/internal/sdk/handler"
)

// ScaffoldingRunTask defines the run task implementation.
type ScaffoldingRunTask struct {
	config handler.Configuration
	logger *log.Logger
}

// NewRunTask instantiates a new ScaffoldingRunTask with a new Logger.
func NewRunTask() *ScaffoldingRunTask {
	return &ScaffoldingRunTask{
		logger: log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
	}
}

// Configure defines the configuration for the server and run task.
// This method is called before the server is initialized.
func (r *ScaffoldingRunTask) Configure(addr string, path string, hmacKey string) {
	r.config = handler.Configuration{
		Addr:    fmt.Sprintf(":%s", addr),
		Path:    path,
		HmacKey: hmacKey,
	}
}

// Below are the 4 potential stages of a run task

// PrePlanStage is executed before the plan is created.
func (r *ScaffoldingRunTask) PrePlanStage(request api.TaskRequest) (*api.TaskResponse, error) {
	// Demo link to show how to set a URL in the response
	referenceURL := fmt.Sprintf("https://example.com/task/%s", request.RunID)

	r.logger.Println("Running Pre-Plan Stage")
	ntr := api.NewTaskResponse()
	runTaskPath, err := request.CreateRunTaskDirectoryStructure()
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return ntr.AddOutcome("create-directory", "Failed to create directory", err.Error(), referenceURL, "failed", api.TagLevelError).
			SetResult(api.TaskFailed, "Pre-Plan Stage Failed: "+err.Error()), err
	}

	// Initialize clients used throughout this stage
	fileManager := helper.NewFileManager()
	tfcClient := helper.NewClient()

	err = fileManager.SaveStructToFile(runTaskPath, "request.json", request)
	if err == nil {
		ntr.AddOutcome("save-request", "Request saved to file successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("save-request", "Failed to save request to file", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	err = tfcClient.GetDataFromAPI(runTaskPath, "run", request)
	if err == nil {
		ntr.AddOutcome("download-run", "Run data downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-run", "Failed to download run from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	err = tfcClient.DownloadConfigurationVersion(runTaskPath, request, fileManager)
	if err == nil {
		ntr.AddOutcome("download-configuration-version", "Configuration version downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-configuration-version", "Failed to download configuration version", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Set the final result based on whether any outcomes were failures
	if ntr.IsPassed() {
		ntr.SetResult(api.TaskPassed, "Pre Plan Stage - Success").
			WithUrl(referenceURL)
	} else {
		ntr.SetResult(api.TaskFailed, "Pre Plan Stage - Failed").
			WithUrl(referenceURL)
	}

	return ntr, nil
}

// PostPlanStage is executed after the plan is created.
func (r *ScaffoldingRunTask) PostPlanStage(request api.TaskRequest) (*api.TaskResponse, error) {
	// Demo link to show how to set a URL in the response
	referenceURL := fmt.Sprintf("https://example.com/task/%s", request.RunID)

	r.logger.Println("Running Post-Plan Stage")
	ntr := api.NewTaskResponse()
	runTaskPath, err := request.CreateRunTaskDirectoryStructure()
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return ntr.AddOutcome("create-directory", "Failed to create directory", err.Error(), referenceURL, "failed", api.TagLevelError).
			SetResult(api.TaskFailed, "Post-Plan Stage Failed: "+err.Error()), err
	}

	// Initialize clients used throughout this stage
	fileManager := helper.NewFileManager()
	tfcClient := helper.NewClient()

	err = tfcClient.DownloadConfigurationVersion(runTaskPath, request, fileManager)
	if err == nil {
		ntr.AddOutcome("download-configuration-version", "Configuration version downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-configuration-version", "Failed to download configuration version", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Save request to JSON file
	err = fileManager.SaveStructToFile(runTaskPath, "request.json", request)
	if err == nil {
		ntr.AddOutcome("save-request", "Request saved to file successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("save-request", "Failed to save request to file", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	err = tfcClient.GetDataFromAPI(runTaskPath, "run", request)
	if err == nil {
		ntr.AddOutcome("download-run", "Run data downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-run", "Failed to download run from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Download Plan as a JSON file
	err = tfcClient.DownloadPlanJson(runTaskPath, request)
	if err == nil {
		ntr.AddOutcome("download-plan-json", "Plan JSON downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-plan-json", "Failed to download plan JSON file", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Plan from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "plan", request)
	if err == nil {
		ntr.AddOutcome("download-plan", "Plan data downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-plan", "Failed to download plan file", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Plan logs
	err = tfcClient.GetLogs(runTaskPath, "plan", request)
	if err == nil {
		ntr.AddOutcome("download-plan-logs", "Plan logs downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-plan-logs", "Failed to get plan logs", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Set the final result based on whether any outcomes were failures
	if ntr.IsPassed() {
		ntr.SetResult(api.TaskPassed, "Post Plan Stage - Success").
			WithUrl(referenceURL)
	} else {
		ntr.SetResult(api.TaskFailed, "Post Plan Stage - Failed").
			WithUrl(referenceURL)
	}

	return ntr, nil
}

// PreApplyStage is executed before the apply is executed.
func (r *ScaffoldingRunTask) PreApplyStage(request api.TaskRequest) (*api.TaskResponse, error) {
	// Demo link to show how to set a URL in the response
	referenceURL := fmt.Sprintf("https://example.com/task/%s", request.RunID)

	r.logger.Println("Running Pre-Apply Stage")
	ntr := api.NewTaskResponse()
	runTaskPath, err := request.CreateRunTaskDirectoryStructure()
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return ntr.AddOutcome("create-directory", "Failed to create directory", err.Error(), referenceURL, "failed", api.TagLevelError).
			SetResult(api.TaskFailed, "Pre-Apply Stage Failed: "+err.Error()), err
	}

	// Initialize clients used throughout this stage
	fileManager := helper.NewFileManager()
	tfcClient := helper.NewClient()

	// Save request to JSON file
	err = fileManager.SaveStructToFile(runTaskPath, "request.json", request)
	if err == nil {
		ntr.AddOutcome("save-request", "Request saved to file successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("save-request", "Failed to save request to file", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Run data from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "run", request)
	if err == nil {
		ntr.AddOutcome("download-run", "Run data downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-run", "Failed to download run from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Policy Checks from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "policy-checks", request)
	if err == nil {
		ntr.AddOutcome("download-policy-checks", "Policy checks downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-policy-checks", "Failed to download policy checks from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Comments from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "comments", request)
	if err == nil {
		ntr.AddOutcome("download-comments", "Comments downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-comments", "Failed to download comments from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Task Stages from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "task-stages", request)
	if err == nil {
		ntr.AddOutcome("download-task-stages", "Task stages downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-task-stages", "Failed to download task stages from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Run Events from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "run-events", request)
	if err == nil {
		ntr.AddOutcome("download-run-events", "Run events downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-run-events", "Failed to download run events from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Set the final result based on whether any outcomes were failures
	if ntr.IsPassed() {
		ntr.SetResult(api.TaskPassed, "Pre Apply Stage - Success").
			WithUrl(referenceURL)
	} else {
		ntr.SetResult(api.TaskFailed, "Pre Apply Stage - Failed").
			WithUrl(referenceURL)
	}

	return ntr, nil
}

// PostApplyStage is executed after the apply is executed.
func (r *ScaffoldingRunTask) PostApplyStage(request api.TaskRequest) (*api.TaskResponse, error) {
	// Demo link to show how to set a URL in the response
	referenceURL := fmt.Sprintf("https://example.com/task/%s", request.RunID)

	r.logger.Println("Running Post-Apply Stage")
	ntr := api.NewTaskResponse()
	runTaskPath, err := request.CreateRunTaskDirectoryStructure()
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return ntr.AddOutcome("create-directory", "Failed to create directory", err.Error(), referenceURL, "failed", api.TagLevelError).
			SetResult(api.TaskFailed, "Post-Apply Stage Failed: "+err.Error()), err
	}

	// Initialize clients used throughout this stage
	fileManager := helper.NewFileManager()
	tfcClient := helper.NewClient()

	// Save request to JSON file
	err = fileManager.SaveStructToFile(runTaskPath, "request.json", request)
	if err == nil {
		ntr.AddOutcome("save-request", "Request saved to file successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("save-request", "Failed to save request to file", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Run data from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "run", request)
	if err == nil {
		ntr.AddOutcome("download-run", "Run data downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-run", "Failed to download run from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Apply data from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "apply", request)
	if err == nil {
		ntr.AddOutcome("download-apply", "Apply data downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-apply", "Failed to download apply from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Apply logs
	err = tfcClient.GetLogs(runTaskPath, "apply", request)
	if err == nil {
		ntr.AddOutcome("download-apply-logs", "Apply logs downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-apply-logs", "Failed to get apply logs", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Policy Checks from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "policy-checks", request)
	if err == nil {
		ntr.AddOutcome("download-policy-checks", "Policy checks downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-policy-checks", "Failed to download policy checks from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Comments from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "comments", request)
	if err == nil {
		ntr.AddOutcome("download-comments", "Comments downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-comments", "Failed to download comments from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Task Stages from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "task-stages", request)
	if err == nil {
		ntr.AddOutcome("download-task-stages", "Task stages downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-task-stages", "Failed to download task stages from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Get the Run Events from API
	err = tfcClient.GetDataFromAPI(runTaskPath, "run-events", request)
	if err == nil {
		ntr.AddOutcome("download-run-events", "Run events downloaded successfully", "", referenceURL, "success", api.TagLevelNone)
	} else {
		ntr.AddOutcome("download-run-events", "Failed to download run events from API", err.Error(), referenceURL, "failed", api.TagLevelError)
	}

	// Set the final result based on whether any outcomes were failures
	if ntr.IsPassed() {
		ntr.SetResult(api.TaskPassed, "Post Apply Stage - Success").
			WithUrl(referenceURL)
	} else {
		ntr.SetResult(api.TaskFailed, "Post Apply Stage - Failed").
			WithUrl(referenceURL)
	}

	return ntr, nil
}
