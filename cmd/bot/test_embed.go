//go:build ignore

package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

//go:embed webapp/index.html webapp/style.css webapp/app.js webapp/data.js
var webappFS embed.FS

func main() {
	// Проверяем что файлы есть
	fmt.Println("=== Files in embed ===")
	fs.WalkDir(webappFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, _ := d.Info()
		fmt.Printf("  %s (%d bytes)\n", path, info.Size())
		return nil
	})

	// Читаем index.html
	data, err := webappFS.ReadFile("index.html")
	if err != nil {
		log.Printf("❌ ReadFile index.html error: %v", err)
	} else {
		fmt.Printf("✅ index.html: %d bytes\n", len(data))
	}

	data, err = webappFS.ReadFile("webapp/index.html")
	if err != nil {
		log.Printf("❌ ReadFile webapp/index.html error: %v", err)
	} else {
		fmt.Printf("✅ webapp/index.html: %d bytes\n", len(data))
	}

	// Запускаем сервер
	http.HandleFunc("/dashboard/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path[len("/dashboard/"):]
		if path == "" {
			path = "index.html"
		}
		data, err := webappFS.ReadFile(path)
		if err != nil {
			log.Printf("❌ Error reading %s: %v", path, err)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	log.Println("Server on :9091")
	http.ListenAndServe(":9091", nil)
}
