package clients

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type StorageClient struct {
	BaseDir      string // absolute or relative directory to store files
	PublicPrefix string // URL prefix where files are served, e.g. "/files"
	BaseURL      string // optional absolute base URL (scheme+host[:port]) used to build file URLs
}

// NewLocalStorage creates a storage client; baseDir will be created if missing.
func NewLocalStorage(baseDir, publicPrefix, baseURL string) (*StorageClient, error) {
	if baseDir == "" {
		baseDir = "./exports"
	}
	if publicPrefix == "" {
		publicPrefix = "/files"
	}

	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to ensure storage dir %q: %w", baseDir, err)
	}

	return &StorageClient{BaseDir: baseDir, PublicPrefix: publicPrefix, BaseURL: baseURL}, nil
}

// Save writes data to baseDir with a unique filename (preserving provided fileName suffix) and returns the filename.
func (s *StorageClient) Save(ctx context.Context, fileName string, data []byte) (string, error) {
	// sanitize provided filename to avoid path traversal
	fileName = filepath.Base(fileName)

	// generate a random prefix to avoid collisions
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("failed to generate file name: %w", err)
	}
	unique := hex.EncodeToString(randBytes)
	final := fmt.Sprintf("%s_%s", unique, fileName)

	path := filepath.Join(s.BaseDir, final)
	// write file atomically
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("failed to finalize file: %w", err)
	}

	return final, nil
}

// GetURL returns public URL for a saved file. If BaseURL is configured, it builds an absolute URL
// (BaseURL + PublicPrefix + / + filename). Otherwise it returns a relative path (PublicPrefix/filename).
func (s *StorageClient) GetURL(fileName string) string {
	// ensure prefix has leading slash and no trailing slash
	prefix := s.PublicPrefix
	if prefix == "" {
		prefix = "/files"
	}

	// remove trailing slash from BaseURL if present
	if s.BaseURL != "" {
		base := s.BaseURL
		if base[len(base)-1] == '/' {
			base = base[:len(base)-1]
		}
		// ensure prefix begins with /
		if prefix[0] != '/' {
			prefix = "/" + prefix
		}
		return fmt.Sprintf("%s%s/%s", base, prefix, fileName)
	}

	if prefix[0] != '/' {
		prefix = "/" + prefix
	}
	return fmt.Sprintf("%s/%s", prefix, fileName)
}

// CleanupOlderThan deletes files older than given duration in base dir.
func (s *StorageClient) CleanupOlderThan(d time.Duration) error {
	now := time.Now()
	return filepath.WalkDir(s.BaseDir, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			return nil
		}
		info, err := de.Info()
		if err != nil {
			return nil
		}
		if now.Sub(info.ModTime()) > d {
			_ = os.Remove(path) // best-effort
		}
		return nil
	})
}
