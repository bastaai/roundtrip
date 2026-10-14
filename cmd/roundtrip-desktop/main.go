// Command roundtrip-desktop runs Roundtrip in a native window (WKWebView on
// macOS, WebView2 on Windows) instead of a browser tab. Build with
// `make desktop`; the tags `desktop,production` are required by Wails.
package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"roundtrip/internal/api"
	"roundtrip/web"
)

func main() {
	config, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	dataDir := filepath.Join(config, "roundtrip")

	// A double-clicked app has no useful working directory, so fall back to a
	// folder in the user's home instead.
	workspace := api.LastWorkspace(dataDir)
	if workspace == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		workspace = filepath.Join(home, "Roundtrip")
	}

	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	handler, err := api.New(api.Config{Workspace: workspace, DataDir: dataDir, Static: dist})
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:     "Roundtrip",
		Width:     1360,
		Height:    860,
		MinWidth:  1024,
		MinHeight: 640,
		// No separate title bar on macOS: the window buttons sit inside the
		// app's own header, which the UI pads and makes draggable (see
		// data-shell in web/src/main.tsx and index.css).
		Mac: &mac.Options{TitleBar: mac.TitleBarHiddenInset()},
		AssetServer: &assetserver.Options{
			// The API only accepts requests addressed to localhost. Requests
			// here never touch the network: they come straight from the app's
			// own webview, so present them as local.
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Host = "localhost"
				r.Header.Del("Origin")
				handler.ServeHTTP(w, r)
			}),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
