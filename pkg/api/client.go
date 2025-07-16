package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"testops-export/pkg/config"
	"testops-export/pkg/models"
)

// Client представляет API клиент для TestOps
type Client struct {
	config        *config.Config
	client        *http.Client
	accessToken   string
	tokenAcquired time.Time
}

// NewClient создает новый API клиент
func NewClient(cfg *config.Config) *Client {
	return &Client{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// getAccessToken получает access_token, если он отсутствует или истёк
func (c *Client) getAccessToken() (string, error) {
	if c.accessToken != "" && time.Since(c.tokenAcquired) < time.Hour {
		return c.accessToken, nil
	}

	endpoint := c.config.BaseURL
	userToken := c.config.Token
	if endpoint == "" || userToken == "" {
		return "", fmt.Errorf("TESTOPS_BASE_URL или TESTOPS_TOKEN не заданы в конфиге")
	}

	tokenURL := endpoint + "/api/uaa/oauth/token"
	data := url.Values{}
	data.Set("grant_type", "apitoken")
	data.Set("scope", "openid")
	data.Set("token", userToken)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса токена: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Expect", "")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка получения токена: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка API токена: %d - %s", resp.StatusCode, string(body))
	}

	var respData struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("ошибка декодирования токена: %v", err)
	}

	c.accessToken = respData.AccessToken
	c.tokenAcquired = time.Now()
	return c.accessToken, nil
}

// RequestExport запрашивает экспорт тесткейсов
func (c *Client) RequestExport(groupID int) (*models.ExportResponse, error) {
	exportReq := createExportRequest(groupID)

	jsonData, err := json.Marshal(exportReq)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга запроса: %v", err)
	}

	url := fmt.Sprintf("%s/api/v2/test-case/bulk/export/csv", c.config.BaseURL)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("ошибка получения access_token: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка API: %d - %s", resp.StatusCode, string(body))
	}

	var exportResp models.ExportResponse
	if err := json.NewDecoder(resp.Body).Decode(&exportResp); err != nil {
		return nil, fmt.Errorf("ошибка декодирования ответа: %v", err)
	}

	return &exportResp, nil
}

// DownloadExport скачивает экспорт по ID
func (c *Client) DownloadExport(exportID int) ([]byte, error) {
	url := fmt.Sprintf("%s/api/export/download/%d", c.config.BaseURL, exportID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса скачивания: %v", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	token, err := c.getAccessToken()
	if err != nil {
		return nil, fmt.Errorf("ошибка получения access_token: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса скачивания: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка скачивания: %d - %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// createExportRequest создает запрос на экспорт для указанной группы
func createExportRequest(groupID int) *models.ExportRequest {
	return &models.ExportRequest{
		Selection: struct {
			ProjectID        int   `json:"projectId"`
			TreeID           int   `json:"treeId"`
			GroupsExclude    []int `json:"groupsExclude"`
			GroupsInclude    []int `json:"groupsInclude"`
			TestCasesExclude []int `json:"testCasesExclude"`
			TestCasesInclude []int `json:"testCasesInclude"`
			Inverted         bool  `json:"inverted"`
			Deleted          bool  `json:"deleted"`
		}{
			ProjectID:        17,
			TreeID:           937,
			GroupsExclude:    []int{},
			GroupsInclude:    []int{groupID},
			TestCasesExclude: []int{},
			TestCasesInclude: []int{},
			Inverted:         false,
			Deleted:          false,
		},
		Mapping: []struct {
			Field          string `json:"field"`
			Name           string `json:"name"`
			ItemsSeparator string `json:"itemsSeparator,omitempty"`
			IntegrationID  int    `json:"integrationId,omitempty"`
			RoleID         int    `json:"roleId,omitempty"`
			CustomFieldID  int    `json:"customFieldId,omitempty"`
		}{
			{Field: "allure_id", Name: "allure_id"},
			{Field: "name", Name: "name"},
			{Field: "full_name", Name: "full_name"},
			{Field: "automated", Name: "automated"},
			{Field: "description", Name: "description"},
			{Field: "precondition", Name: "precondition"},
			{Field: "expected_result", Name: "expected_result"},
			{Field: "status", Name: "status"},
			{Field: "scenario", Name: "scenario"},
			{Field: "tag", Name: "tag", ItemsSeparator: ","},
			{Field: "link", Name: "link"},
			{Field: "example", Name: "example"},
			{Field: "parameter", Name: "parameter", ItemsSeparator: ","},
			{Field: "issue_integration", Name: "Gitlab", IntegrationID: 2, ItemsSeparator: ","},
			{Field: "issue_integration", Name: "Интеграция с WB Youtrack", IntegrationID: 1, ItemsSeparator: ","},
			{Field: "role", Name: "Lead", RoleID: -2, ItemsSeparator: ","},
			{Field: "role", Name: "Owner", RoleID: -1, ItemsSeparator: ","},
			{Field: "role", Name: "AutoQA", RoleID: 2, ItemsSeparator: ","},
			{Field: "role", Name: "Author", RoleID: 3, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Suite", CustomFieldID: -5, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Component", CustomFieldID: -4, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Story", CustomFieldID: -3, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Feature", CustomFieldID: -2, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Epic", CustomFieldID: -1, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Sub-Element", CustomFieldID: 8, ItemsSeparator: ","},
			{Field: "custom_field", Name: "Sub-Suite", CustomFieldID: 9, ItemsSeparator: ","},
		},
		ColumnSeparator: ";",
		IncludeHeaders:  true,
		Name:            "report.csv",
	}
}
