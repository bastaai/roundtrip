package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"roundtrip/internal/api"
	"roundtrip/internal/runner"
	"roundtrip/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		os.Exit(runner.Main(os.Args[2:], os.Stdout, os.Stderr, runner.Terminal(os.Stdout)))
	}
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	noOpen := flag.Bool("no-open", false, "don't open the browser on start")
	data := flag.String("data", "", "directory for per-machine data (environments, history); defaults to the user config directory")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: roundtrip [flags] [workspace]\n       roundtrip run [flags] [path ...]\n\nWithout a command the app starts and opens in the browser. workspace is the\ndirectory (usually a git repo) holding collections/. Defaults to the one\nopened last, then the current directory.\n\n`roundtrip run` runs requests and their tests in the terminal; see\n`roundtrip run -h`.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	dataDir := *data
	if dataDir == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		dataDir = filepath.Join(config, "roundtrip")
	}

	// An explicit argument wins, then whatever was open last time, then the current directory.
	workspace := flag.Arg(0)
	if workspace == "" {
		workspace = api.LastWorkspace(dataDir)
	}
	dist, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	handler, err := api.New(api.Config{Workspace: workspace, DataDir: dataDir, Static: dist})
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String()
	log.Printf("roundtrip on %s", url)
	if !*noOpen {
		openBrowser(url)
	}
	log.Fatal(http.Serve(ln, handler))
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		exec.Command("xdg-open", url).Start()
	}
}
