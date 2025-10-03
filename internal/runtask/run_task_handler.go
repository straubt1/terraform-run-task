// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package runtask

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/straubt1/terraform-run-task/internal/helper"
	"github.com/straubt1/terraform-run-task/internal/sdk/api"
	"github.com/straubt1/terraform-run-task/internal/sdk/handler"
)

// HandleRequests sets up the HTTP server and routes for handling TFC requests and health checks.
// This function only runs once when the task server starts.
func HandleRequests(task *ScaffoldingRunTask) {
	r := mux.NewRouter()

	// Printing the HMAC key should be avoided in a production environment!
	task.logger.Println("Registering " + task.config.Path + " route" + " with HMAC key set to " + task.config.HmacKey)
	r.HandleFunc(task.config.Path, handleTFCRequestWrapper(task, sendTFCCallbackResponse())).
		Methods(http.MethodPost)

	task.logger.Println("Registering /healthcheck route")
	r.HandleFunc("/healthcheck", healthcheck(task)).
		Methods(http.MethodGet)

	task.logger.Printf("Starting server on port %s", task.config.Addr)
	err := http.ListenAndServe(task.config.Addr, r)
	if err != nil {
		return
	}
}

// Healthcheck endpoint, required to verify the service is running and to create the Run Task in HCP Terraform.
func healthcheck(task *ScaffoldingRunTask) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		task.logger.Println("/healthcheck called")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]string{"status": "available"})
		if err != nil {
			return
		}
	}
}

// This is the entry point for a Run Task request from HCP Terraform.
// It validates the request, determines the stage, and calls the appropriate stage function.
func handleTFCRequestWrapper(task *ScaffoldingRunTask, callback func(http.ResponseWriter, *http.Request, api.Request, *ScaffoldingRunTask, *handler.CallbackBuilder)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		task.logger.Println(task.config.Path + " called")

		// Parse request
		var runTaskReq api.Request
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			task.logger.Println("Error occurred while parsing the request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(reqBody, &runTaskReq)
		if err != nil {
			task.logger.Println("Error occurred while parsing the request")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		task.logger.Println("Run Task Stage:", runTaskReq.Stage, "for workspace:", runTaskReq.WorkspaceName, "and run ID:", runTaskReq.RunID)

		requestSha := r.Header.Get(handler.HeaderTaskSignature)

		if requestSha != "" && task.config.HmacKey == "" {
			task.logger.Printf("Received a request for %s with a signature but this server cannot validate signed requests\n", r.URL)
			http.Error(w, "Unexpected x-tfc-task-signature header", http.StatusBadRequest)
			return
		}

		if requestSha == "" && task.config.HmacKey != "" {
			task.logger.Printf("Received an unsigned request for %s\n", r.URL)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if requestSha != "" {
			// Calculate expected HMAC
			verified, err := handler.VerifyHMAC(reqBody, []byte(r.Header.Get(handler.HeaderTaskSignature)), []byte(task.config.HmacKey))

			if err != nil {
				task.logger.Println("Unable to verify given HMAC key")
				http.Error(w, "Error verifying signed request", http.StatusInternalServerError)
				return
			}

			if !verified {
				task.logger.Println("Received an unauthorized request")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			task.logger.Println("Successfully verified HMAC signature")
		}

		// IsEndpointValidation returns true if the Request is from the
		// run task service to validate this API endpoint.
		// This occurs when you create the Run Task in Organization
		// Callers should immediately return an HTTP 200 status code for these requests.
		if runTaskReq.IsEndpointValidation() {
			task.logger.Println("Successfully validated TFC request - runTaskReq.IsEndpointValidation()")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the appropriate stage function based on the stage in the request
		var stageResponse *handler.CallbackBuilder
		var stageError error
		if runTaskReq.Stage == api.PrePlan {
			stageResponse, stageError = task.PrePlanStage(runTaskReq)
		} else if runTaskReq.Stage == api.PostPlan {
			stageResponse, stageError = task.PostPlanStage(runTaskReq)
		} else if runTaskReq.Stage == api.PreApply {
			stageResponse, stageError = task.PreApplyStage(runTaskReq)
		} else if runTaskReq.Stage == api.PostApply {
			stageResponse, stageError = task.PostApplyStage(runTaskReq)
		} else {
			task.logger.Println("Run task is running in an unknown stage:", runTaskReq.Stage)
			http.Error(w, "Bad Request: unknown stage "+runTaskReq.Stage, http.StatusBadRequest)
			return
		}

		if stageError != nil {
			task.logger.Println("Error occurred during stage execution:", stageError.Error())
			http.Error(w, "Error during stage execution:"+stageError.Error(), http.StatusInternalServerError)
			return
		}

		// Call the original function to send the response back to TFC with the stage result
		callback(w, r, runTaskReq, task, stageResponse)
	}
}

// Function to reply back to HCP Terraform with the task result for the Stage.
func sendTFCCallbackResponse() func(w http.ResponseWriter, r *http.Request, reqBody api.Request, task *ScaffoldingRunTask, cbBuilder *handler.CallbackBuilder) {
	return func(w http.ResponseWriter, r *http.Request, reqBody api.Request, task *ScaffoldingRunTask, cbBuilder *handler.CallbackBuilder) {
		respBody, err := cbBuilder.MarshallJSON()
		if err != nil {
			task.logger.Println("Unable to marshall callback response to TFC")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Send PATCH callback response to TFC
		tfcClient := helper.NewClient()
		request, err := tfcClient.SendGenericHttpRequest(reqBody.TaskResultCallbackURL, http.MethodPatch, reqBody.AccessToken, respBody)
		if request != nil {
			_ = r.Body.Close()
		}
		if err != nil {
			task.logger.Println("Error occurred while sending the callback response to TFC")
			http.Error(w, "Bad Request:"+err.Error(), http.StatusNotFound)
			return
		}

		task.logger.Println("Sent run task response to TFC")
	}
}
