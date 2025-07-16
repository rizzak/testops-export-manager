package models

import "time"

// ExportRequest представляет запрос на экспорт тесткейсов
type ExportRequest struct {
	Selection struct {
		ProjectID        int   `json:"projectId"`
		TreeID           int   `json:"treeId"`
		GroupsExclude    []int `json:"groupsExclude"`
		GroupsInclude    []int `json:"groupsInclude"`
		TestCasesExclude []int `json:"testCasesExclude"`
		TestCasesInclude []int `json:"testCasesInclude"`
		Inverted         bool  `json:"inverted"`
		Deleted          bool  `json:"deleted"`
	} `json:"selection"`
	Mapping []struct {
		Field          string `json:"field"`
		Name           string `json:"name"`
		ItemsSeparator string `json:"itemsSeparator,omitempty"`
		IntegrationID  int    `json:"integrationId,omitempty"`
		RoleID         int    `json:"roleId,omitempty"`
		CustomFieldID  int    `json:"customFieldId,omitempty"`
	} `json:"mapping"`
	ColumnSeparator string `json:"columnSeparator"`
	IncludeHeaders  bool   `json:"includeHeaders"`
	Name            string `json:"name"`
}

// ExportResponse представляет ответ на запрос экспорта
type ExportResponse struct {
	ID int `json:"id"`
}

// ExportConfig представляет конфигурацию экспорта для группы
type ExportConfig struct {
	GroupID   int
	GroupName string
}

// ExportFile представляет файл экспорта
type ExportFile struct {
	Name          string
	Size          int64
	ModifiedTime  time.Time
	FormattedSize string
	FormattedDate string
}

// PageData представляет данные для веб-страницы
type PageData struct {
	Files      []ExportFile
	TotalFiles string
	TotalSize  string
	LastExport string
}
