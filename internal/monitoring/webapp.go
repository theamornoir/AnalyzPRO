package monitoring

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed webapp_files/index.html webapp_files/style.css webapp_files/app.js
var monitoringWebappFS embed.FS

// ServeWebApp отдаёт статический веб-апп мониторинга из embed-файлов.
// Обслуживается по префиксу /monitoring/ маршрутизатором бота.
func ServeWebApp(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Path
	if filePath == "/monitoring/" || filePath == "/monitoring" || filePath == "/" {
		filePath = "index.html"
	} else {
		filePath = strings.TrimPrefix(filePath, "/monitoring/")
	}

	switch {
	case filePath == "index.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case filePath == "style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case filePath == "app.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	default:
		http.NotFound(w, r)
		return
	}

	data, err := monitoringWebappFS.ReadFile("webapp_files/" + filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Не кэшируем — упрощает разработку/обновление веб-аппа.
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// webappFilesFS возвращает поддерево встроенных файлов (для тестов/проверки).
func webappFilesFS() (fs.FS, error) {
	return fs.Sub(monitoringWebappFS, "webapp_files")
}
