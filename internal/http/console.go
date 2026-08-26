package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var consoleFiles embed.FS

func (r *Router) consoleRedirect(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "/console", http.StatusTemporaryRedirect)
}

func (r *Router) consoleHandler() http.Handler {
	assets, err := fs.Sub(consoleFiles, "web")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/console", http.FileServer(http.FS(assets)))
}
