package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"testops-export/pkg/config"
	"testops-export/pkg/export"
	"testops-export/pkg/models"
)

// Server представляет веб-сервер
type Server struct {
	config  *config.Config
	manager *export.Manager
	httpSrv *http.Server
}

// NewServer создает новый веб-сервер
func NewServer(manager *export.Manager) *Server {
	return &Server{
		config:  manager.Config(),
		manager: manager,
	}
}

// Start запускает веб-сервер
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/export", s.handleExport)
	mux.HandleFunc("/download/", s.handleDownload)

	s.httpSrv = &http.Server{
		Addr:    ":" + s.config.WebPort,
		Handler: mux,
	}

	log.Printf("Веб-сервер запущен на порту %s", s.config.WebPort)
	log.Printf("Откройте http://localhost:%s в браузере", s.config.WebPort)

	return s.httpSrv.ListenAndServe()
}

// Shutdown останавливает веб-сервер корректно
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

// handleIndex обрабатывает главную страницу
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Получаем project_id из query
	var files []models.ExportFile
	var err error
	projectIDStr := r.URL.Query().Get("project_id")
	var selectedProjectID int64
	if projectIDStr != "" {
		fmt.Sscanf(projectIDStr, "%d", &selectedProjectID)
		files, err = s.manager.GetExportFiles(selectedProjectID)
	} else {
		files, err = s.manager.GetExportFiles()
	}
	if err != nil {
		http.Error(w, "Ошибка чтения файлов", http.StatusInternalServerError)
		return
	}

	// Собираем список всех проектов из конфига (Projects), чтобы показывать даже те, по которым ещё нет экспортов
	projectMap := make(map[int64]struct{})
	for _, p := range s.config.Projects {
		if p.ProjectID != 0 {
			projectMap[p.ProjectID] = struct{}{}
		}
	}
	var projects []models.ProjectInfo
	for id := range projectMap {
		projects = append(projects, models.ProjectInfo{ID: id, Name: fmt.Sprintf("%d", id)})
	}

	// Подсчитываем статистику
	totalSize := int64(0)
	var lastExportTime time.Time

	for _, file := range files {
		totalSize += file.Size
		if file.ModifiedTime.After(lastExportTime) {
			lastExportTime = file.ModifiedTime
		}
	}

	lastExport := "Нет"
	if !lastExportTime.IsZero() {
		lastExport = lastExportTime.Format("02.01.2006 15:04:05")
	}

	data := models.PageData{
		Files:             files,
		TotalFiles:        fmt.Sprintf("%d", len(files)),
		TotalSize:         s.manager.FormatFileSize(totalSize),
		LastExport:        lastExport,
		Projects:          projects,
		SelectedProjectID: selectedProjectID,
	}

	tmpl, err := template.New("index").Funcs(template.FuncMap{
		"toJson": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}).Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Ошибка шаблона", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

// handleExport обрабатывает запрос на экспорт
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	type req struct {
		ProjectID int64 `json:"project_id"`
	}
	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Неверный формат запроса", http.StatusBadRequest)
		return
	}

	if body.ProjectID == 0 {
		go s.manager.PerformExportParallel()
	} else {
		go s.manager.PerformExportForProjectParallel(body.ProjectID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Экспорт запущен"})
}

// handleDownload обрабатывает скачивание файлов
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[len("/download/"):]
	if filename == "" {
		http.Error(w, "Имя файла не указано", http.StatusBadRequest)
		return
	}

	// Проверяем, что имя файла не содержит подозрительные символы
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		http.Error(w, "Доступ запрещен", http.StatusForbidden)
		return
	}

	data, err := s.manager.DownloadExportFile(filename)
	if err != nil {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "text/csv")
	w.Write(data)
}

// HTML шаблон для веб-интерфейса
const htmlTemplate = `
<!DOCTYPE html>
<html lang="ru">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>TestOps Export Manager</title>
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>📊</text></svg>">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 2.5em;
            font-weight: 300;
        }
        .header p {
            margin: 10px 0 0 0;
            opacity: 0.9;
            font-size: 1.1em;
        }
        .content {
            padding: 30px;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }
        .stat-card {
            background: #f8f9fa;
            padding: 20px;
            border-radius: 8px;
            text-align: center;
            border-left: 4px solid #667eea;
        }
        .stat-number {
            font-size: 2em;
            font-weight: bold;
            color: #667eea;
        }
        .stat-label {
            color: #6c757d;
            margin-top: 5px;
        }
        .actions {
            margin-bottom: 30px;
            text-align: center;
        }
        .btn {
            background: #667eea;
            color: white;
            border: none;
            padding: 12px 24px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 1em;
            text-decoration: none;
            display: inline-block;
            margin: 5px;
            transition: background-color 0.3s;
        }
        .btn:hover {
            background: #5a6fd8;
        }
        .btn-secondary {
            background: #6c757d;
        }
        .btn-secondary:hover {
            background: #5a6268;
        }
        .btn:disabled {
            background: #c0c4cc;
            color: #f8f9fa;
            cursor: not-allowed;
            opacity: 0.7;
        }
        .exports-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        .exports-table th,
        .exports-table td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #dee2e6;
        }
        .exports-table th {
            background-color: #f8f9fa;
            font-weight: 600;
            color: #495057;
        }
        .exports-table tr:hover {
            background-color: #f8f9fa;
        }
        .download-link {
            color: #667eea;
            text-decoration: none;
            font-weight: 500;
        }
        .download-link:hover {
            text-decoration: underline;
        }
        .empty-state {
            text-align: center;
            padding: 60px 20px;
            color: #6c757d;
        }
        .empty-state h3 {
            margin-bottom: 10px;
            color: #495057;
        }
        .status-indicator {
            display: inline-block;
            width: 8px;
            height: 8px;
            border-radius: 50%;
            margin-right: 8px;
        }
        .status-active {
            background-color: #28a745;
        }
        .status-inactive {
            background-color: #dc3545;
        }
        .header-icon {
            font-size: 3em;
            margin-bottom: 10px;
            display: block;
        }
        .project-select-label {
            font-weight: 500;
            margin-right: 8px;
            color: #495057;
        }
        .project-select {
            padding: 8px 16px;
            border-radius: 6px;
            border: 1px solid #ced4da;
            font-size: 1em;
            background: #f8f9fa;
            color: #495057;
            transition: border-color 0.2s;
            outline: none;
            margin-right: 10px;
        }
        .project-select:focus {
            border-color: #667eea;
            background: #fff;
        }
        @media (max-width: 768px) {
            .header h1 {
                font-size: 2em;
            }
            .content {
                padding: 20px;
            }
            .exports-table {
                font-size: 0.9em;
            }
            .exports-table th,
            .exports-table td {
                padding: 8px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>TestOps Export Manager</h1>
            <p>Управление экспортами тесткейсов</p>
        </div>
        
        <div class="content">
            <div class="stats">
                <div class="stat-card">
                    <div class="stat-number">{{.TotalFiles}}</div>
                    <div class="stat-label">Всего экспортов</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">{{.TotalSize}}</div>
                    <div class="stat-label">Общий размер</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">{{.LastExport}}</div>
                    <div class="stat-label">Последний экспорт</div>
                </div>
                <div class="stat-card">
                    <div class="status-indicator status-active"></div>
                    <div class="stat-label">Статус сервиса</div>
                </div>
            </div>

            <div class="actions">
                <form id="projectForm" style="display:inline;">
                    <label for="projectSelect" class="project-select-label">Проект:</label>
                    <select id="projectSelect" name="project_id" class="project-select" onchange="document.getElementById('projectForm').submit()">
                        <option value="" {{if eq .SelectedProjectID 0}}selected{{end}}>Все</option>
                        {{range .Projects}}
                        <option value="{{.ID}}" {{if eq $.SelectedProjectID .ID}}selected{{end}}>{{.Name}}</option>
                        {{end}}
                    </select>
                </form>
                <button id="exportBtn" class="btn" {{if eq .SelectedProjectID 0}}disabled{{end}}>Запустить экспорт сейчас</button>
                <button type="button" class="btn btn-secondary" onclick="location.reload();">Обновить</button>
            </div>

            <div id="exportStatus" style="text-align:center; margin-bottom:20px; color:#28a745; display:none;"></div>

            <div style="text-align:center; margin-bottom:20px; color:#6c757d; font-size:0.9em;">
                <p>⏰ Автоматический экспорт выполняется в 10:00 UTC (13:00 MSK) по будням</p>
                <p>📅 Все временные метки отображаются в UTC</p>
                <p>🔄 При ошибках система автоматически повторит попытку до 10 раз с интервалом 15-150 минут</p>
            </div>

            {{if .Files}}
            <table class="exports-table">
                <thead>
                    <tr>
                        <th>Файл</th>
                        <th>Размер</th>
                        <th>Дата создания</th>
                        <th>Действия</th>
                    </tr>
                </thead>
                <tbody id="exportsTbody">
                    <!-- JS будет рендерить сюда -->
                </tbody>
            </table>
            <button id="showMoreBtn" class="btn" style="display:none;">Показать ещё</button>
            {{else}}
            <div class="empty-state">
                <h3>Экспорты не найдены</h3>
                <p>Запустите первый экспорт, чтобы увидеть файлы здесь.</p>
            </div>
            {{end}}
        </div>
    </div>
    <script>
    document.getElementById('exportBtn').onclick = function() {
        var btn = this;
        if (btn.disabled) return;
        btn.disabled = true;
        btn.textContent = 'Экспортируется...';
        var projectId = Number(document.getElementById('projectSelect').value);
        fetch('/export', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ project_id: projectId })
        })
            .then(r => r.json())
            .then(data => {
                document.getElementById('exportStatus').style.display = 'block';
                document.getElementById('exportStatus').textContent = 'Экспорт запущен! Обновите страницу через минуту.';
                btn.textContent = 'Запустить экспорт сейчас';
                btn.disabled = false;
            })
            .catch(() => {
                document.getElementById('exportStatus').style.display = 'block';
                document.getElementById('exportStatus').style.color = '#dc3545';
                document.getElementById('exportStatus').textContent = 'Ошибка запуска экспорта!';
                btn.textContent = 'Запустить экспорт сейчас';
                btn.disabled = false;
            });
    };

const allFiles = {{ toJson .Files }};
let shown = 10;

function renderFiles() {
    const tbody = document.getElementById('exportsTbody');
    tbody.innerHTML = '';
    for (let i = 0; i < Math.min(shown, allFiles.length); i++) {
        const f = allFiles[i];
        tbody.innerHTML += '<tr>' +
            '<td>' + f.Name + '</td>' +
            '<td>' + f.FormattedSize + '</td>' +
            '<td>' + f.FormattedDate + '</td>' +
            '<td><a href="/download/' + f.Name + '" class="download-link">Скачать</a></td>' +
        '</tr>';
    }
    document.getElementById('showMoreBtn').style.display = shown < allFiles.length ? '' : 'none';
}

document.getElementById('showMoreBtn').onclick = function() {
    shown += 10;
    renderFiles();
};

window.onload = function() {
    renderFiles();
    if (allFiles.length > 10) document.getElementById('showMoreBtn').style.display = '';
};
    </script>
</body>
</html>
`
