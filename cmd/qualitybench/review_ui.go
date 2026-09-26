package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

//go:embed review-ui/index.html
var reviewHTML []byte

//go:embed review-ui/model.js
var reviewModel []byte

//go:embed review-ui/app.js
var reviewApp []byte

func regexpBlindID() *regexp.Regexp { return regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`) }

// Only the reviewer-safe CSV and explicitly listed PNGs are read. Never serve
// the evidence directory: it contains the blind key and configuration names.
func newReviewHandler(dir string) (http.Handler, error) {
	return newReviewHandlerWithReport(dir, "")
}

type reviewSheets struct {
	ids    []string
	paths  map[string]string
	digest hash.Hash
}

func loadReviewSheets(dir string) (*reviewSheets, error) {
	f, err := os.Open(filepath.Join(dir, "ratings_template.csv")) // #nosec G304 -- operator-selected bundle
	if err != nil {
		return nil, fmt.Errorf("open reviewer template: %w", err)
	}
	rows, err := csv.NewReader(f).ReadAll()
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("read reviewer template: %w", err)
	}
	if len(rows) < 2 || len(rows[0]) == 0 || rows[0][0] != "blind_id" {
		return nil, fmt.Errorf("reviewer template must contain a blind_id column and at least one sheet")
	}
	root, err := filepath.EvalSymlinks(filepath.Join(dir, "sheets"))
	if err != nil {
		return nil, fmt.Errorf("resolve contact sheets: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	validID := regexpBlindID()
	ids := make([]string, 0, len(rows)-1)
	paths := map[string]string{}
	digest := sha256.New()
	for _, row := range rows[1:] {
		id := row[0]
		if !validID.MatchString(id) || paths[id] != "" {
			return nil, fmt.Errorf("invalid or duplicate blind ID %q", id)
		}
		path, err := filepath.EvalSymlinks(filepath.Join(root, id+".png"))
		if err != nil {
			return nil, fmt.Errorf("sheet %s: %w", id, err)
		}
		if filepath.Dir(path) != root {
			return nil, fmt.Errorf("sheet %s resolves outside the sheets directory", id)
		}
		file, err := os.Open(path) // #nosec G304 -- validated allowlisted sheet
		if err != nil {
			return nil, err
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = file.Close()
			return nil, fmt.Errorf("sheet %s is not a readable regular file", id)
		}
		_, _ = digest.Write([]byte(id + "\x00"))
		_, err = io.Copy(digest, file)
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("hash sheet %s: %w", id, err)
		}
		ids = append(ids, id)
		paths[id] = path
	}
	return &reviewSheets{ids, paths, digest}, nil
}

func newReviewHandlerWithReport(dir, reportPath string) (http.Handler, error) {
	sheets, err := loadReviewSheets(dir)
	if err != nil {
		return nil, err
	}
	ids, paths, digest := sheets.ids, sheets.paths, sheets.digest
	pairs := map[string]string{}
	prompts := map[string]string{}
	failedRuns := 0
	if reportPath != "" {
		data, err := os.ReadFile(reportPath) // #nosec G304 -- operator-selected report; only safe fields are exposed
		if err != nil {
			return nil, err
		}
		var report qualitybench.Report
		if err := json.Unmarshal(data, &report); err != nil {
			return nil, err
		}
		groups := map[string][]qualitybench.Evidence{}
		for _, ev := range report.Evidence {
			if ev.Error != "" {
				failedRuns++
				continue
			}
			if paths[ev.BlindID] == "" {
				return nil, fmt.Errorf("report sheet %s is not in the reviewer bundle", ev.BlindID)
			}
			prompts[ev.BlindID] = ev.Request.Brief.Prompt
			group := fmt.Sprintf("%s\x00%s\x00%d", ev.Request.Brief.ID, ev.Request.Template.Name, ev.Request.Repetition)
			groups[group] = append(groups[group], ev)
		}
		if len(prompts) != len(ids) {
			return nil, fmt.Errorf("report and reviewer bundle contain different sheets")
		}
		for _, group := range groups {
			if len(group) == 2 && group[0].Request.Configuration != group[1].Request.Configuration {
				pairs[group[0].BlindID] = group[1].BlindID
				pairs[group[1].BlindID] = group[0].BlindID
			}
		}
	}
	metadata, err := json.Marshal(struct {
		Pairs   map[string]string
		Prompts map[string]string
	}{pairs, prompts})
	if err != nil {
		return nil, err
	}
	_, _ = digest.Write(metadata)
	manifest, err := json.Marshal(struct {
		Dataset    string            `json:"dataset"`
		IDs        []string          `json:"ids"`
		Pairs      map[string]string `json:"pairs"`
		Prompts    map[string]string `json:"prompts"`
		FailedRuns int               `json:"failed_runs"`
	}{hex.EncodeToString(digest.Sum(nil)), ids, pairs, prompts, failedRuns})
	if err != nil {
		return nil, err
	}
	return blindReviewHTTPHandler(paths, manifest), nil
}

func blindReviewHTTPHandler(paths map[string]string, manifest []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "loopback host required", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "read-only review server", http.StatusMethodNotAllowed)
			return
		}
		var body []byte
		var contentType string
		switch r.URL.Path {
		case "/":
			body, contentType = reviewHTML, "text/html; charset=utf-8"
		case "/model.js":
			body, contentType = reviewModel, "text/javascript; charset=utf-8"
		case "/app.js":
			body, contentType = reviewApp, "text/javascript; charset=utf-8"
		case "/manifest":
			body, contentType = manifest, "application/json"
		default:
			id := strings.TrimPrefix(r.URL.Path, "/sheet/")
			if strings.HasPrefix(r.URL.Path, "/sheet/") && paths[id] != "" {
				http.ServeFile(w, r, paths[id])
				return
			}
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}
	})
}

func serveReviewUI(dir, address, reportPath string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("-listen must be a loopback IP:port, for example 127.0.0.1:8765")
	}
	handler, err := newReviewHandlerWithReport(dir, reportPath)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("start review UI: %w", err)
	}
	defer func() { _ = listener.Close() }()
	fmt.Printf("Blind review UI: http://%s\nKeep this process running. Ratings stay in your browser; download a backup regularly.\n", listener.Addr())
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	return server.Serve(listener)
}
