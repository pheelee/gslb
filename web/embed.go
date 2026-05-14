// Package web provides embedded frontend files
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded filesystem
func FS() fs.FS {
	f, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return f
}

// indexHTML holds the parsed index.html bytes for direct serving.
var indexHTML []byte

func init() {
	fsys := FS()
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		panic("web: failed to read index.html: " + err.Error())
	}
	indexHTML = data
}

// serveIndex writes index.html directly, bypassing http.FileServer which would
// redirect any request whose path ends with "/index.html" to "./".
func serveIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	c.Abort()
}

// Handler serves the embedded frontend with SPA routing
func Handler() gin.HandlerFunc {
	fsys := FS()
	fileServer := http.FileServer(http.FS(fsys))

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// API routes pass through
		if len(path) >= 4 && path[0:4] == "/api" {
			c.Next()
			return
		}

		// Health check routes pass through
		if path == "/health" || path == "/ready" {
			c.Next()
			return
		}

		// Try to open the requested file
		file, err := fsys.Open(path[1:])
		if err != nil {
			// File not found: serve index.html for SPA routing
			serveIndex(c)
			return
		}
		defer file.Close()

		// Check if it's a directory or the root
		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			serveIndex(c)
			return
		}

		// Static asset — let the file server handle content-type, caching, etc.
		// Strip the leading "/" so the file server path matches the embedded FS root.
		http.StripPrefix("/", fileServer).ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}
