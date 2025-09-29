// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package runtask

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/straubt1/terraform-run-task/internal/sdk/api"
	"github.com/straubt1/terraform-run-task/internal/sdk/handler"
)

// HandleRequests sets up the HTTP server and routes for handling TFC requests and health checks.
// This function only runs once when the task server starts.
func HandleRequests(task *ScaffoldingRunTask) {
	r := mux.NewRouter()

	task.logger.Println("Registering " + task.config.Path + " route")
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

// INFO: 2025/09/23 12:13:28 /runtask called
// INFO: 2025/09/23 12:13:28 handleTFCRequestWrapper() - start
// INFO: 2025/09/23 12:13:28 Successfully verified request
// INFO: 2025/09/23 12:13:28 sendTFCCallbackResponse() - start
// INFO: 2025/09/23 12:13:28 Sent run task response to TFC
// INFO: 2025/09/23 12:13:28 sendTFCCallbackResponse() - end
// INFO: 2025/09/23 12:13:28 handleTFCRequestWrapper() - end
func handleTFCRequestWrapper(task *ScaffoldingRunTask, original func(http.ResponseWriter, *http.Request, api.Request, *ScaffoldingRunTask, *handler.CallbackBuilder)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		task.logger.Println(task.config.Path + " called")
		task.logger.Println("handleTFCRequestWrapper() - start")

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
		task.logger.Println("Run Task Stage:", runTaskReq.Stage)

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
		}

		// IsEndpointValidation returns true if the Request is from the
		// run task service to validate this API endpoint. Callers should
		// immediately return an HTTP 200 status code for these requests.
		if runTaskReq.IsEndpointValidation() {
			task.logger.Println("Successfully validated TFC request - runTaskReq.IsEndpointValidation()")
			w.WriteHeader(http.StatusOK)
			return
		}

		var stageResponse *handler.CallbackBuilder
		var stageError error
		// Call the appropriate stage function based on the stage in the request
		// var stageResponse, stageError *handler.CallbackBuilder error
		// stageResponse, stageError
		if runTaskReq.Stage == api.PrePlan {
			task.logger.Println("Running in the pre-plan stage")
			stageResponse, stageError = task.PrePlanStage(runTaskReq)
		} else if runTaskReq.Stage == api.PostPlan {
			task.logger.Println("Running in the post-plan stage")
			stageResponse, stageError = task.PostPlanStage(runTaskReq)
		} else if runTaskReq.Stage == api.PreApply {
			task.logger.Println("Running in the pre-apply stage")
			stageResponse, stageError = task.PreApplyStage(runTaskReq)
		} else if runTaskReq.Stage == api.PostApply {
			task.logger.Println("Running in the post-apply stage")
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

		// callbackResp, err := task.VerifyRequest(runTaskReq)
		// if err != nil {
		// 	task.logger.Println("Error occurred during run task request verification")
		// 	http.Error(w, "Error during run task request verification:"+err.Error(), http.StatusInternalServerError)
		// 	return
		// }

		// // Get TFC Plan if the task is running in the post-plan or pre-apply stages
		// if runTaskReq.Stage == api.PostPlan || runTaskReq.Stage == api.PreApply {
		// 	plan, err := retrieveTFCPlan(runTaskReq)

		// 	if err != nil {
		// 		task.logger.Println("Error occurred while retrieving plan from TFC")
		// 		http.Error(w, "Bad Request: "+err.Error(), http.StatusNotFound)
		// 		return
		// 	}
		// 	task.logger.Println("Successfully retrieved plan from TFC")

		// 	callbackResp, err = task.VerifyPlan(runTaskReq, plan)
		// 	if err != nil {
		// 		task.logger.Println("Error occurred while verifying plan")
		// 		http.Error(w, "Error verifying plan:"+err.Error(), http.StatusInternalServerError)
		// 		return
		// 	}
		// }

		original(w, r, runTaskReq, task, stageResponse)
		// original(w, r, runTaskReq, task, callbackResp)

		task.logger.Println("handleTFCRequestWrapper() - end")
	}
}

// These functions reply back to HCP Terraform with the task result for the Stage.
func sendTFCCallbackResponse() func(w http.ResponseWriter, r *http.Request, reqBody api.Request, task *ScaffoldingRunTask, cbBuilder *handler.CallbackBuilder) {

	return func(w http.ResponseWriter, r *http.Request, reqBody api.Request, task *ScaffoldingRunTask, cbBuilder *handler.CallbackBuilder) {

		respBody, err := cbBuilder.MarshallJSON()
		if err != nil {
			task.logger.Println("Unable to marshall callback response to TFC")
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Send PATCH callback response to TFC
		request, err := sendTFCRequest(reqBody.TaskResultCallbackURL, http.MethodPatch, reqBody.AccessToken, respBody)
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

func retrieveTFCPlan(req api.Request) (tfjson.Plan, error) {

	// Call TFC to get plan
	resp, err := sendTFCRequest(req.PlanJSONAPIURL, "GET", req.AccessToken, nil)
	if err != nil {
		return tfjson.Plan{}, err
	}

	var tfPlan tfjson.Plan

	if resp == nil {
		return tfPlan, fmt.Errorf("expected Terraform plan from TFC but received none")
	}

	respBody, err := io.ReadAll(resp.Body)

	_ = resp.Body.Close()

	if err != nil {
		return tfPlan, err
	}

	err = json.Unmarshal(respBody, &tfPlan)

	if err != nil {
		return tfPlan, err
	}

	return tfPlan, nil
}

// sends a generic HTTP request to TFC with the required headers
func sendTFCRequest(url string, method string, accessToken string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		os.Exit(1)
	}

	// Required headers to send to TFC
	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return http.DefaultClient.Do(req)
}

func saveRequestToFile(filePath string, request api.Request) error {
	// filePath := fmt.Sprintf("%s/request.json", runTaskPath)
	file, err := os.Create(filePath)
	if err != nil {
		// r.logger.Println("Error creating request.json:", err)
		// return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to create request.json: " + err.Error()), err
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(request); err != nil {
		// r.logger.Println("Error encoding request to JSON:", err)
		// return handler.NewCallbackBuilder(api.TaskFailed).WithMessage("Failed to encode request to JSON: " + err.Error()), err
		return err
	}
	return nil
}

func downloadPlanFile(filePath string, request api.Request) error {
	url := strings.TrimSuffix(request.PlanJSONAPIURL, "/json-output")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	// this token doesnt have the proper permissions
	req.Header.Set("Authorization", "Bearer "+"request.AccessToken")
	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download plan file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Unmarshal and marshal with indentation for pretty JSON
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		return fmt.Errorf("failed to pretty print JSON: %w", err)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = out.Write(prettyJSON.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func downloadPlanJsonFile(filePath string, request api.Request) error {
	req, err := http.NewRequest("GET", request.PlanJSONAPIURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+request.AccessToken)
	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download plan file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Unmarshal and marshal with indentation for pretty JSON
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		return fmt.Errorf("failed to pretty print JSON: %w", err)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = out.Write(prettyJSON.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// extractTarGz extracts a tar.gz file to a directory with the specified ID
func extractTarGz(directory, filename, id string) error {
	// Create the target directory path
	targetDir := filepath.Join(directory, id)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Open the tar.gz file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files from the tar archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Create the full path for the file
		targetPath := filepath.Join(targetDir, header.Name)

		// Ensure the target path is within the target directory (security check)
		if !filepath.HasPrefix(targetPath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			// Create file
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}

	return nil
}
