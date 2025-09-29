// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package runtask

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/straubt1/terraform-run-task/internal/sdk/api"
	"github.com/straubt1/terraform-run-task/internal/sdk/handler"
)

// ScaffoldingRunTask defines the run task implementation.
type ScaffoldingRunTask struct {
	config handler.Configuration
	logger *log.Logger
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

// VerifyRequest defines custom run task integration logic.
// This method is called after the run task receives and validates the run task request from TFC.
func (r *ScaffoldingRunTask) VerifyRequest(request api.Request) (*handler.CallbackBuilder, error) {

	// Run custom verification logic
	r.logger.Println("Successfully verified request")
	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Custom Passed Message 1"), nil
}

// VerifyPlan defines custom integration logic for verifying the run's plan from TFC.
// This method is only called if the run task is running in the post-plan or pre-apply stages
// and if VerifyRequest returns a nil response with no error.
func (r *ScaffoldingRunTask) VerifyPlan(request api.Request, plan tfjson.Plan) (*handler.CallbackBuilder, error) {
	r.logger.Println("Successfully verified plan")
	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Custom Passed Message 2"), nil
}

// New Funcs
//
// config version ID, is speculative, run message, VCS info,
func (r *ScaffoldingRunTask) PrePlanStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Pre-Plan Stage")
	runTaskPath := fmt.Sprintf("%s/%s/%s/%s", ".", request.WorkspaceName, request.RunID, api.PrePlan)
	r.logger.Printf("Run task path: %s", runTaskPath)
	err := os.MkdirAll(runTaskPath, os.ModePerm)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Pre-Plan Stage Failed: " + err.Error()), err
	}

	err = saveRequestToFile(fmt.Sprintf("%s/request.json", runTaskPath), request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	req, err := http.NewRequest(http.MethodGet, request.ConfigurationVersionDownloadURL, nil)
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		os.Exit(1)
	}

	// Required headers to send to TFC
	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	req.Header.Set("Authorization", "Bearer "+request.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.logger.Println("Error sending request:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to send request: " + err.Error()), err
	}
	defer resp.Body.Close()

	outFilePath := fmt.Sprintf("%s/%s.tar.gz", runTaskPath, request.ConfigurationVersionID)
	outFile, err := os.Create(outFilePath)
	if err != nil {
		r.logger.Println("Error creating configuration.tar.gz:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to create configuration.tar.gz: " + err.Error()), err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		r.logger.Println("Error saving tar archive to disk:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save tar archive: " + err.Error()), err
	}
	// Unpack the downloaded tar.gz file in the same folder
	outFile.Seek(0, 0) // Ensure file pointer is at the beginning
	outFile.Close()    // Close so we can reopen for reading

	err = extractTarGz(runTaskPath, outFilePath, request.ConfigurationVersionID)
	if err != nil {
		r.logger.Println("Error extracting tar.gz:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to extract tar.gz: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Pre-Plan Stage Passed"), nil
}

func (r *ScaffoldingRunTask) PostPlanStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Post-Plan Stage")
	runTaskPath := fmt.Sprintf("%s/%s/%s/%s", ".", request.WorkspaceName, request.RunID, api.PostPlan)
	r.logger.Printf("Run task path: %s", runTaskPath)

	// Create directory structure
	err := os.MkdirAll(runTaskPath, os.ModePerm)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Pre-Plan Stage Failed: " + err.Error()), err
	}

	// Save request to JSON file
	err = saveRequestToFile(fmt.Sprintf("%s/request.json", runTaskPath), request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	// Download Plan JSON file
	err = downloadPlanJsonFile(filepath.Join(runTaskPath, "plan_json.json"), request)
	// err = downloadPlanJsonFile(fmt.Sprintf("%s/plan.json", runTaskPath), request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download plan JSON file: " + err.Error()), err
	}

	// Download Plan file
	err = downloadPlanFile(filepath.Join(runTaskPath, "plan_api.json"), request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to download plan file: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Post-Plan Stage Passed"), nil
}

func (r *ScaffoldingRunTask) PreApplyStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Pre-Apply Stage")

	runTaskPath := fmt.Sprintf("%s/%s/%s/%s", ".", request.WorkspaceName, request.RunID, api.PreApply)
	r.logger.Printf("Run task path: %s", runTaskPath)

	// Create directory structure
	err := os.MkdirAll(runTaskPath, os.ModePerm)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Pre-Plan Stage Failed: " + err.Error()), err
	}

	// Save request to JSON file
	err = saveRequestToFile(fmt.Sprintf("%s/request.json", runTaskPath), request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Pre-Apply Stage Passed"), nil
}

func (r *ScaffoldingRunTask) PostApplyStage(request api.Request) (*handler.CallbackBuilder, error) {
	r.logger.Println("Running Post-Apply Stage")

	runTaskPath := fmt.Sprintf("%s/%s/%s/%s", ".", request.WorkspaceName, request.RunID, api.PostApply)
	r.logger.Printf("Run task path: %s", runTaskPath)

	// Create directory structure
	err := os.MkdirAll(runTaskPath, os.ModePerm)
	if err != nil {
		r.logger.Println("Error creating directory:", err)
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Pre-Plan Stage Failed: " + err.Error()), err
	}

	// Save request to JSON file
	err = saveRequestToFile(fmt.Sprintf("%s/request.json", runTaskPath), request)
	if err != nil {
		return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to save request to file: " + err.Error()), err
	}

	return handler.NewCallbackBuilder(api.TaskPassed).WithMessage("Post-Apply Stage Passed"), nil
}

// NewRunTask instantiates a new ScaffoldingRunTask with a new Logger.
func NewRunTask() *ScaffoldingRunTask {
	return &ScaffoldingRunTask{
		logger: log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime),
	}
}
