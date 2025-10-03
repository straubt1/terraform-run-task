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

	"github.com/straubt1/terraform-run-task/internal/sdk/api"
)

// Saves the run task request (JSON) to a file
func saveRequestToFile(outputDirectory string, request api.Request) error {
	filePath := filepath.Join(outputDirectory, "request.json")
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(request); err != nil {
		return fmt.Errorf("failed to encode request to JSON: %w", err)
	}
	return nil
}

func downloadConfigurationVersion(outputDirectory string, request api.Request) error {
	cvFolder := filepath.Join(outputDirectory, request.ConfigurationVersionID)
	cvFile := filepath.Join(outputDirectory, request.ConfigurationVersionID+".tar.gz")
	req, err := http.NewRequest(http.MethodGet, request.ConfigurationVersionDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("client: could not create request: %w", err)
	}

	// Required headers to send to TFC
	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	req.Header.Set("Authorization", "Bearer "+request.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	outFile, err := os.Create(cvFile)
	if err != nil {
		return fmt.Errorf("failed to create configuration.tar.gz: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save tar archive: %w", err)
	}
	// Unpack the downloaded tar.gz file in the same folder
	outFile.Seek(0, 0) // Ensure file pointer is at the beginning
	outFile.Close()    // Close so we can reopen for reading

	err = extractTarGz(cvFile, cvFolder, request.ConfigurationVersionID)
	if err != nil {
		return fmt.Errorf("failed to extract tar.gz: %w", err)
	}
	return nil
}

// extractTarGz extracts a tar.gz file to a directory with the specified ID
func extractTarGz(archiveFile, targetDir, id string) error {
	// Create the target directory path
	// targetDir := filepath.Join(directory, id)

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Open the tar.gz file
	file, err := os.Open(archiveFile)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", archiveFile, err)
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

// Download Plan as a JSON file
func downloadPlanJson(outputDirectory string, request api.Request) error {
	filePath := filepath.Join(outputDirectory, "plan_json.json")
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

// Get the plan logs from the API and save it to a file
// Reads the plan_api.json file to get the log URL which has an embedded archivist token
func getPlanLogs(outputDirectory string, request api.Request) error {
	filePath := filepath.Join(outputDirectory, "plan_logs.txt")
	planApiFile := filepath.Join(outputDirectory, "plan_api.json")
	// Read and parse the plan API file
	planApiData, err := os.ReadFile(planApiFile)
	if err != nil {
		// If file is not found, skip this step
		// TODO: better logging
		// return fmt.Errorf("failed to read plan API file: %w", err)
		return nil
	}

	var planApiResponse struct {
		Data struct {
			Attributes struct {
				LogReadURL string `json:"log-read-url"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(planApiData, &planApiResponse); err != nil {
		return fmt.Errorf("failed to parse plan API JSON: %w", err)
	}

	logURL := planApiResponse.Data.Attributes.LogReadURL
	if logURL == "" {
		return fmt.Errorf("log-read-url not found in plan API response")
	}

	// Download the logs from the URL
	req, err := http.NewRequest("GET", logURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Save logs to file
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write logs to file: %w", err)
	}

	return nil

}

// Get data (run, plan, apply) from the API and save it to files
func getDataFromAPI(outputDirectory string, dataType string, request api.Request) error {
	token := getPermissiveToken()
	hostname := getHostname(request)
	if token == "" {
		return nil // If no token, skip this step
	}
	apiPath := dataType
	if dataType == "run" { // no sub-path for run
		apiPath = ""
	}
	filePath := filepath.Join(outputDirectory, fmt.Sprintf("%s_api.json", dataType))

	url := fmt.Sprintf("%s/api/v2/runs/%s/%s", hostname, request.RunID, apiPath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s request: %w", dataType, err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
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

// Get the apply logs from the API and save it to a file
// Reads the apply_api.json file to get the log URL which has an embedded archivist token
func getApplyLogs(outputDirectory string, request api.Request) error {
	filePath := filepath.Join(outputDirectory, "apply_logs.txt")
	applyApiFile := filepath.Join(outputDirectory, "apply_api.json")
	// Read and parse the apply API file
	applyApiData, err := os.ReadFile(applyApiFile)
	if err != nil {
		// If file is not found, skip this step
		// TODO: better logging
		// return fmt.Errorf("failed to read plan API file: %w", err)
		return nil
	}

	var planApiResponse struct {
		Data struct {
			Attributes struct {
				LogReadURL string `json:"log-read-url"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(applyApiData, &planApiResponse); err != nil {
		return fmt.Errorf("failed to parse plan API JSON: %w", err)
	}

	logURL := planApiResponse.Data.Attributes.LogReadURL
	if logURL == "" {
		return fmt.Errorf("log-read-url not found in plan API response")
	}

	// Download the logs from the URL
	req, err := http.NewRequest("GET", logURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Save logs to file
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write logs to file: %w", err)
	}

	return nil

}

// sends a generic HTTP request with the required headers
func sendGenericHttpRequest(url string, method string, accessToken string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		fmt.Printf("client: could not create request: %s\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return http.DefaultClient.Do(req)
}

// get a permissive token from the environment variable, empty if not set
func getPermissiveToken() string {
	return os.Getenv("PERMISSIVE_ACCESS_TOKEN")
}

// get the hostname from the task request callback URL
func getHostname(request api.Request) string {
	// Extract hostname from the TaskResultCallbackURL
	// e.g., "https://app.terraform.io/api/v2/task-results/..." -> "https://app.terraform.io"
	hostname := ""
	if idx := strings.Index(request.TaskResultCallbackURL, "/api/"); idx != -1 {
		hostname = request.TaskResultCallbackURL[:idx]
	}
	return hostname
}
