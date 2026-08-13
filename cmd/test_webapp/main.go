package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
)

//go:embed webapp/index.html webapp/style.css webapp/app.js webapp/data.js
var webappFS embed.FS

func main() {
	// Check files
	fmt.Println("=== Files ===")
	fs.WalkDir(webappFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil { return err }
		info, _ := d.Info()
		fmt.Printf("  %s (%d bytes)\n", path, info.Size())
		return nil
	})
	
	data, err := webappFS.ReadFile("index.html")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("✅ index.html: %d bytes\n", len(data))
	}
	
	// Handler
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard/", 301)
	})
	
	http.HandleFunc("/dashboard/", func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Path[len("/dashboard/"):]
		if filePath == "" { filePath = "index.html" }
		filePath = "webapp/" + filePath
		
		switch {
		case strings.HasSuffix(filePath, "index.html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case strings.HasSuffix(filePath, "style.css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		case strings.HasSuffix(filePath, ".js"):
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		default:
			http.NotFound(w, r)
			return
		}
		
		data, err := webappFS.ReadFile(filePath)
		if err != nil {
			fmt.Printf("❌ Read %s: %v\n", filePath, err)
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	})
	
	log.Println("Server on :9092")
	http.ListenAndServe(":9092", nil)
}
