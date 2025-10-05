package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/straubt1/terraform-run-task/internal/sdk/api"
)

// Client handles Terraform Cloud API interactions
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new TFC API client
func NewClient() *Client {
	return &Client{
		httpClient: http.DefaultClient,
	}
}

// DownloadConfigurationVersion downloads and extracts a configuration version
func (c *Client) DownloadConfigurationVersion(outputDirectory string, request api.TaskRequest, extractor ArchiveExtractor) error {
	cvFolder := filepath.Join(outputDirectory, request.ConfigurationVersionID)
	cvFile := filepath.Join(outputDirectory, request.ConfigurationVersionID+".tar.gz")

	// Download the configuration version
	if err := c.downloadFile(request.ConfigurationVersionDownloadURL, cvFile, request.AccessToken); err != nil {
		return fmt.Errorf("failed to download configuration version: %w", err)
	}

	// Extract the downloaded tar.gz file
	if err := extractor.ExtractTarGz(cvFile, cvFolder, request.ConfigurationVersionID); err != nil {
		return fmt.Errorf("failed to extract tar.gz: %w", err)
	}

	return nil
}

// DownloadPlanJson downloads the plan as a JSON file
func (c *Client) DownloadPlanJson(outputDirectory string, request api.TaskRequest) error {
	filePath := filepath.Join(outputDirectory, "plan_json.json")

	body, err := c.makeAPIRequest("GET", request.PlanJSONAPIURL, request.AccessToken, nil)
	if err != nil {
		return fmt.Errorf("failed to download plan JSON: %w", err)
	}

	return c.savePrettyJSON(body, filePath)
}

// GetDataFromAPI retrieves data from the TFC API and saves it to a file
func (c *Client) GetDataFromAPI(outputDirectory string, dataType string, request api.TaskRequest) error {
	token := c.GetPermissiveToken()
	if token == "" {
		return nil // If no token, skip this step
	}

	hostname := c.GetHostname(request)
	apiPath := dataType
	if dataType == "run" { // no sub-path for run
		apiPath = ""
	}

	filePath := filepath.Join(outputDirectory, fmt.Sprintf("%s_api.json", dataType))
	url := fmt.Sprintf("%s/api/v2/runs/%s/%s", hostname, request.RunID, apiPath)

	body, err := c.makeAPIRequest("GET", url, token, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s data: %w", dataType, err)
	}

	return c.savePrettyJSON(body, filePath)
}

// GetLogs retrieves logs from the API based on the API response file
func (c *Client) GetLogs(outputDirectory, logType string, request api.TaskRequest) error {
	apiFileName := fmt.Sprintf("%s_api.json", logType)
	logFileName := fmt.Sprintf("%s_logs.txt", logType)

	logURL, err := c.extractLogURL(filepath.Join(outputDirectory, apiFileName))
	if err != nil {
		return err // This will return nil if file not found, which is expected behavior
	}

	logFilePath := filepath.Join(outputDirectory, logFileName)
	return c.downloadFile(logURL, logFilePath, "")
}

// GetPermissiveToken gets a permissive token from the environment variable
func (c *Client) GetPermissiveToken() string {
	return os.Getenv("TERRAFORM_API_TOKEN")
}

// GetHostname extracts the hostname from the task request callback URL
func (c *Client) GetHostname(request api.TaskRequest) string {
	// Extract hostname from the TaskResultCallbackURL
	// e.g., "https://app.terraform.io/api/v2/task-results/..." -> "https://app.terraform.io"
	if idx := strings.Index(request.TaskResultCallbackURL, "/api/"); idx != -1 {
		return request.TaskResultCallbackURL[:idx]
	}
	return ""
}

// SendGenericHttpRequest sends a generic HTTP request with the required headers
func (c *Client) SendGenericHttpRequest(url string, method string, accessToken string, body []byte) (*http.Response, error) {
	return c.makeHTTPRequest(method, url, accessToken, body)
}

// ArchiveExtractor interface for extracting archives (allows for easier testing)
type ArchiveExtractor interface {
	ExtractTarGz(archiveFile, targetDir, id string) error
}

// Private helper methods

func (c *Client) downloadFile(url, filePath, accessToken string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (c *Client) makeAPIRequest(method, url, accessToken string, body []byte) ([]byte, error) {
	resp, err := c.makeHTTPRequest(method, url, accessToken, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) makeHTTPRequest(method, url, accessToken string, body []byte) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", api.JsonApiMediaTypeHeader)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	return c.httpClient.Do(req)
}

func (c *Client) savePrettyJSON(data []byte, filePath string) error {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		return fmt.Errorf("failed to pretty print JSON: %w", err)
	}

	outFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = outFile.Write(prettyJSON.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (c *Client) extractLogURL(apiFilePath string) (string, error) {
	apiData, err := os.ReadFile(apiFilePath)
	if err != nil {
		// If file is not found, skip this step (expected behavior)
		return "", nil
	}

	var apiResponse struct {
		Data struct {
			Attributes struct {
				LogReadURL string `json:"log-read-url"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(apiData, &apiResponse); err != nil {
		return "", fmt.Errorf("failed to parse API JSON: %w", err)
	}

	logURL := apiResponse.Data.Attributes.LogReadURL
	if logURL == "" {
		return "", fmt.Errorf("log-read-url not found in API response")
	}

	return logURL, nil
}
