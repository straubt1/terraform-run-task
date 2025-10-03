// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package runtask

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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
func (r *ScaffoldingRunTask) PrePlanStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Pre-Plan Stage")
	runTaskPath, err := createRunTaskDirectoryStructure(request, api.PrePlan)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Pre-Plan Stage Failed: " + err.Error()), err
	}

	err = saveRequestToFile(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	err = getDataFromAPI(runTaskPath, "run", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download run from API: " + err.Error()), err
	}

	err = downloadConfigurationVersion(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download configuration version: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Pre-Plan Stage Passed"), nil
}

// PostPlanStage is executed after the plan is created.
func (r *ScaffoldingRunTask) PostPlanStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Post-Plan Stage")
	runTaskPath, err := createRunTaskDirectoryStructure(request, api.PostPlan)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Post-Plan Stage Failed: " + err.Error()), err
	}

	err = downloadConfigurationVersion(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download configuration version: " + err.Error()), err
	}

	// Save request to JSON file
	err = saveRequestToFile(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	err = getDataFromAPI(runTaskPath, "run", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download run from API: " + err.Error()), err
	}

	// Download Plan as a JSON file
	err = downloadPlanJson(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download plan JSON file: " + err.Error()), err
	}

	// Get the Plan from API
	err = getDataFromAPI(runTaskPath, "plan", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download plan file: " + err.Error()), err
	}

	// Get the Plan logs
	err = getPlanLogs(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to get plan logs: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Post-Plan Stage Passed"), nil
}

// PreApplyStage is executed before the apply is executed.
func (r *ScaffoldingRunTask) PreApplyStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Pre-Apply Stage")
	runTaskPath, err := createRunTaskDirectoryStructure(request, api.PreApply)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Pre-Apply Stage Failed: " + err.Error()), err
	}

	// Save request to JSON file
	err = saveRequestToFile(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	// Get the Run data from API
	err = getDataFromAPI(runTaskPath, "run", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download run from API: " + err.Error()), err
	}

	// Get the Policy Checks from API
	err = getDataFromAPI(runTaskPath, "policy-checks", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download policy checks from API: " + err.Error()), err
	}

	// Get the Comments from API
	err = getDataFromAPI(runTaskPath, "comments", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download comments from API: " + err.Error()), err
	}

	// Get the Task Stages from API
	err = getDataFromAPI(runTaskPath, "task-stages", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download task stages from API: " + err.Error()), err
	}

	// Get the Run Events from API
	err = getDataFromAPI(runTaskPath, "run-events", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download run events from API: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Pre-Apply Stage Passed"), nil
}

// PostApplyStage is executed after the apply is executed.
func (r *ScaffoldingRunTask) PostApplyStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Post-Apply Stage")
	runTaskPath, err := createRunTaskDirectoryStructure(request, api.PostApply)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Post-Apply Stage Failed: " + err.Error()), err
	}

	// Save request to JSON file
	err = saveRequestToFile(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	// Get the Run data from API
	err = getDataFromAPI(runTaskPath, "run", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download run from API: " + err.Error()), err
	}

	// Get the Apply data from API
	err = getDataFromAPI(runTaskPath, "apply", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download apply from API: " + err.Error()), err
	}

	// Get the Apply logs
	err = getApplyLogs(runTaskPath, request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to get apply logs: " + err.Error()), err
	}

	// Get the Policy Checks from API
	err = getDataFromAPI(runTaskPath, "policy-checks", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download policy checks from API: " + err.Error()), err
	}

	// Get the Comments from API
	err = getDataFromAPI(runTaskPath, "comments", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download comments from API: " + err.Error()), err
	}

	// Get the Task Stages from API
	err = getDataFromAPI(runTaskPath, "task-stages", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download task stages from API: " + err.Error()), err
	}

	// Get the Run Events from API
	err = getDataFromAPI(runTaskPath, "run-events", request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download run events from API: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Post-Apply Stage Passed"), nil
}

// Create the directory structure for the run task based on workspace, run ID, and stage
func createRunTaskDirectoryStructure(request api.Request, stage string) (string, error) {
	// Prefix the stage folder with a number to make it easier to read
	var stageFolder string
	switch stage {
	case api.PrePlan:
		stageFolder = "1_" + stage
	case api.PostPlan:
		stageFolder = "2_" + stage
	case api.PreApply:
		stageFolder = "3_" + stage
	case api.PostApply:
		stageFolder = "4_" + stage
	default:
		stageFolder = stage
	}
	path := filepath.Join(".", request.WorkspaceName, request.RunID, stageFolder)
	err := os.MkdirAll(path, os.ModePerm)
	return path, err
}
