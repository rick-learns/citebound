package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// UI serves the built web UI from files, which is rooted at Vite's output
// directory. "/" is index.html. Directories are never listed. Vite names each
// file under assets/ by a hash of its content, so those are cached for a
// year; everything else is revalidated.
//
// If files is nil, the binary was built without the UI, and every path is a
// 404 that says how to build it.
//
// Mount it at "/", not "GET /": that would conflict with the API's "/api/"
// catch-all, so this checks the method itself.
func UI(files fs.FS) http.Handler {
	return onlyGet(serveUI(files))
}

func onlyGet(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "use GET", http.StatusMethodNotAllowed)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func serveUI(files fs.FS) http.Handler {
	if files == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("This binary was built without the web UI. Build it with `make build`,\n" +
				"or run the dev server with `cd web && npm run dev`. The API is under /api/.\n"))
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		info, err := fs.Stat(files, name)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		b, err := fs.ReadFile(files, name)
		if err != nil {
			http.Error(w, "could not read "+name, http.StatusInternalServerError)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
	})
}
