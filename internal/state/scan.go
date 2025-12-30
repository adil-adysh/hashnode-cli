package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)


// LocalPost represents a raw markdown file found on disk
type LocalPost struct {
       Path     string
       Slug     string // Derived from filename (e.g. "posts/hello.md" -> "hello")
       Checksum string // SHA256 hash of content
       Content  string // The actual text content
}

// ScanDirectory walks the folder and indexes all .md files
func ScanDirectory(root string) (map[string]LocalPost, error) {
       posts := make(map[string]LocalPost)

       err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
	       if err != nil {
		       return err
	       }
	       // Ignore hidden directories (.git, .hnsync, node_modules)
	       if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
		       return filepath.SkipDir
	       }
	       if d.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
		       return nil
	       }
	       // Calculate Hash & Read Content
	       content, hash, err := readFileAndHash(path)
	       if err != nil {
		       return fmt.Errorf("scan error %s: %w", path, err)
	       }
	       // Derive slug from filename
	       filename := filepath.Base(path)
	       slug := strings.TrimSuffix(filename, filepath.Ext(filename))
	       posts[slug] = LocalPost{
		       Path:     path,
		       Slug:     slug,
		       Checksum: hash,
		       Content:  content,
	       }
	       return nil
       })

       return posts, err
}

func readFileAndHash(path string) (string, string, error) {
       f, err := os.Open(path)
       if err != nil {
	       return "", "", err
       }
       defer f.Close()

       hasher := sha256.New()
       // Read file into hasher AND string builder simultaneously
       content, err := io.ReadAll(io.TeeReader(f, hasher))
       if err != nil {
	       return "", "", err
       }

       return string(content), hex.EncodeToString(hasher.Sum(nil)), nil
}
