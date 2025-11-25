package clients

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetURL_AbsoluteAndRelative(t *testing.T) {
	tmpDir := t.TempDir()

	c, err := NewLocalStorage(tmpDir, "/files", "http://example.com:8060")
	if err != nil {
		t.Fatalf("failed create storage: %v", err)
	}

	got := c.GetURL("a.xlsx")
	want := "http://example.com:8060/files/a.xlsx"
	if got != want {
		t.Fatalf("expected %s; got %s", want, got)
	}

	// without base url
	c2, _ := NewLocalStorage(tmpDir, "/files", "")
	if got2 := c2.GetURL("b.xlsx"); got2 != "/files/b.xlsx" {
		t.Fatalf("expected /files/b.xlsx; got %s", got2)
	}
}

func TestSaveAndServeFileHandler(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := NewLocalStorage(tmpDir, "/files", "")
	if err != nil {
		t.Fatalf("storage init: %v", err)
	}

	content := []byte("hello world")
	saved, err := c.Save(context.Background(), "report 1.xlsx", content)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// create handler as in main: serve file from BaseDir
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/files/")
		path := filepath.Join(c.BaseDir, file)
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}
		if idx := strings.IndexByte(file, '_'); idx >= 0 {
			file = file[idx+1:]
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+file+"\"")
		http.ServeFile(w, r, path)
	})

	ts := httptest.NewServer(h)
	defer ts.Close()

	// c.GetURL returns a relative path like /files/<saved>, so request via ts.URL
	resp, err := http.Get(ts.URL + c.GetURL(saved))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("bad status: %d", resp.StatusCode)
	}

	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "report 1.xlsx") {
		t.Fatalf("expected Content-Disposition with original filename, got %s", cd)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(content) {
		t.Fatalf("content mismatch: %s", string(body))
	}
}
