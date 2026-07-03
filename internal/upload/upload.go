// Package upload wires up tus resumable uploads. Files land in a staging dir,
// and on completion are moved into the media library and registered as pending
// videos (the user then picks a quality and processes them).
package upload

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
)

// New returns the tus HTTP handler. mediaDir is the container path to the media
// library (must be writable); uploads stage under mediaDir/.uploads.
func New(pool *pgxpool.Pool, mediaDir string) (http.Handler, error) {
	staging := filepath.Join(mediaDir, ".uploads")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, fmt.Errorf("create staging dir: %w", err)
	}

	store := filestore.New(staging)
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)

	h, err := tusd.NewHandler(tusd.Config{
		BasePath:              "/api/upload/",
		StoreComposer:         composer,
		NotifyCompleteUploads: true,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		for ev := range h.CompleteUploads {
			onComplete(pool, mediaDir, staging, ev.Upload)
		}
	}()

	return h, nil
}

func onComplete(pool *pgxpool.Pool, mediaDir, staging string, info tusd.FileInfo) {
	name := filepath.Base(info.MetaData["filename"])
	// filepath.Base never returns a path separator, but it does pass "." and
	// ".." through — either would escape the library dir.
	if name == "" || name == "." || name == ".." || name == "/" {
		name = info.ID
	}

	// VR uploads (marked by the client) go to their own subdir and never show
	// up in the shared film library.
	isVR := info.MetaData["vr"] == "1"
	dstDir := mediaDir
	if isVR {
		dstDir = filepath.Join(mediaDir, "vr")
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			log.Printf("upload %s: vr dir: %v", info.ID, err)
			return
		}
	}

	srcPath := filepath.Join(staging, info.ID)
	dstPath := uniquePath(filepath.Join(dstDir, name))
	if err := os.Rename(srcPath, dstPath); err != nil {
		log.Printf("upload %s: move to library failed: %v", info.ID, err)
		return
	}
	_ = os.Remove(srcPath + ".info") // tus metadata sidecar

	title := strings.TrimSuffix(filepath.Base(dstPath), filepath.Ext(dstPath))
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO videos (title, file_path, is_vr) VALUES ($1, $2, $3)`,
		title, dstPath, isVR); err != nil {
		log.Printf("upload %s: register failed: %v", info.ID, err)
		return
	}
	log.Printf("upload complete: %s", dstPath)
}

// uniquePath appends " (N)" before the extension until the path is free, so an
// upload never silently overwrites an existing library file.
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		p := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
}
