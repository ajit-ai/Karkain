// Package regserver provides a local, in-process Karkain registry server used
// for hermetic testing of the registry client and protocol. It implements the
// same v1 HTTP protocol that a production Karkain registry would expose, so the
// client code is exercised end-to-end without any external network dependency.
//
// This is a TEST server, not a production deployment. PUBLIC REGISTRY
// DEPLOYMENT REMAINS FUTURE WORK (see P2-GATE3-REGISTRY-REPORT.md).
package regserver

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Package is a package stored in the server's in-memory store.
type Package struct {
	Name        string
	Versions    map[string][]byte // version -> tarball bytes (gzipped tar)
	Description string
	Author      string
	License     string
	Repository  string
	Keywords    []string
	Latest      string
}

// Server is an in-memory Karkain registry implementing the v1 protocol.
type Server struct {
	mu       sync.Mutex
	packages map[string]*Package
	ts       *httptest.Server
}

// New creates an empty registry server.
func New() *Server {
	return &Server{packages: make(map[string]*Package)}
}

// Handler returns the root http.Handler implementing the v1 protocol.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/packages", s.handleListSearch)
	mux.HandleFunc("/v1/packages/", s.handlePackage)
	return mux
}

// URL returns an httptest.Server URL bound to this store.
func (s *Server) URL() string {
	if s.ts == nil {
		s.ts = httptest.NewServer(s.Handler())
	}
	return s.ts.URL
}

// Close shuts down an httptest server created via URL().
func (s *Server) Close() {
	if s.ts != nil {
		s.ts.Close()
		s.ts = nil
	}
}

// Add registers a package with one or more published version tarballs and
// metadata. versions maps version -> source directory whose contents become the
// gzipped tar returned on download.
func (s *Server) Add(name string, versions map[string]string, meta PublishMeta) (*Package, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := &Package{
		Name:        name,
		Versions:    make(map[string][]byte),
		Description: meta.Description,
		Author:      meta.Author,
		License:     meta.License,
		Repository:  meta.Repository,
		Keywords:    meta.Keywords,
	}
	var latest string
	for v, srcDir := range versions {
		data, err := tarballDir(srcDir)
		if err != nil {
			return nil, err
		}
		p.Versions[v] = data
		if v > latest {
			latest = v
		}
	}
	p.Latest = latest
	s.packages[name] = p
	return p, nil
}

// PublishMeta carries optional package metadata for Add.
type PublishMeta struct {
	Description string
	Author      string
	License     string
	Repository  string
	Keywords    []string
}

func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// handleListSearch serves GET /v1/packages[?q=...].
func (s *Server) handleListSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query().Get("q")
	s.mu.Lock()
	names := make([]string, 0, len(s.packages))
	for n := range s.packages {
		names = append(names, n)
	}
	sort.Strings(names)
	type hit struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Author      string `json:"author"`
	}
	var results []hit
	for _, n := range names {
		p := s.packages[n]
		if q != "" && !strings.Contains(n, q) && !strings.Contains(p.Description, q) {
			continue
		}
		results = append(results, hit{Name: n, Version: p.Latest, Description: p.Description, Author: p.Author})
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handlePackage routes /v1/packages/{name} and /v1/packages/{name}/{version}/*
// including download and publish.
func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v1/packages/")
	parts := strings.SplitN(rest, "/", 2)
	name := parts[0]

	switch r.Method {
	case http.MethodGet:
		if len(parts) == 2 && strings.HasSuffix(parts[1], "/download") {
			version := strings.TrimSuffix(parts[1], "/download")
			s.handleDownload(w, name, version)
			return
		}
		s.handleInfo(w, name)
	case http.MethodPost:
		if len(parts) == 2 && strings.HasSuffix(parts[1], "/publish") {
			s.handlePublish(w, r, name)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, name string) {
	s.mu.Lock()
	p, ok := s.packages[name]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	var versions []string
	for v := range p.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":        p.Name,
		"latest":      p.Latest,
		"description": p.Description,
		"author":      p.Author,
		"license":     p.License,
		"repository":  p.Repository,
		"keywords":    p.Keywords,
		"versions":    versions,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, name, version string) {
	s.mu.Lock()
	p, ok := s.packages[name]
	var data []byte
	if ok {
		data = p.Versions[version]
	}
	s.mu.Unlock()
	if !ok || data == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("X-Karkain-Sha256", sha256Of(data))
	w.Write(data)
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request, name string) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	version := r.Header.Get("Karkain-Version")
	if version == "" {
		http.Error(w, "missing Karkain-Version", http.StatusBadRequest)
		return
	}
	expected := strings.TrimPrefix(r.Header.Get("X-Karkain-Sha256"), "sha256:")
	if expected != "" && !strings.EqualFold(expected, sha256Of(data)) {
		http.Error(w, "checksum mismatch", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	p := s.packages[name]
	if p == nil {
		p = &Package{Name: name, Versions: make(map[string][]byte)}
		s.packages[name] = p
	}
	p.Versions[version] = data
	if version > p.Latest {
		p.Latest = version
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

// tarballDir builds a gzipped tar of the directory contents with relative,
// forward-slash names for cross-platform determinism.
func tarballDir(dir string) ([]byte, error) {
	var bb bytesBuffer
	gz := gzip.NewWriter(&bb)
	tw := tar.NewWriter(gz)

	base := filepath.Clean(dir)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeDir, Mode: 0755})
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		hdr := &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0644, Size: info.Size()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return bb.b, nil
}

type bytesBuffer struct {
	b []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.b = append(b.b, p...)
	return len(p), nil
}
