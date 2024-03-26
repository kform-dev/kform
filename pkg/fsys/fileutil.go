package fsys

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/henderiw/logger/log"
)

var ErrPathNoDirectory = errors.New("invalid path, path needs a specific directory")

func EnsureDir(ctx context.Context, elems ...string) error {
	log := log.FromContext(ctx)
	fp := filepath.Join(elems...)
	//log.Info("ensure dir", "path", fp)
	fpInfo, err := os.Stat(fp)
	if err != nil {
		if err := os.MkdirAll(fp, 0755); err != nil {
			log.Error("cannot create dir", slog.String("error", err.Error()))
			return err
		}
	} else {
		if !fpInfo.IsDir() {
			return fmt.Errorf("expecting directory")
		}
	}
	return nil
}

// NormalizeDir returns full absolute directory path of the
// passed directory or an error. This function cleans up paths
// such as current directory (.), relative directories (..), or
// multiple separators.
func NormalizeDir(dirPath string) (string, error) {
	if !IsDir(dirPath) {
		return "", fmt.Errorf("invalid directory argument: %s", dirPath)
	}
	return filepath.Abs(dirPath)
}

func IsDir(dir string) bool {
	if f, err := os.Stat(dir); err == nil {
		if f.IsDir() {
			return true
		}
	}
	return false
}

// fileExists returns true if a file at path already exists;
// false otherwise.
func FileExists(path string) bool {
	f, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !f.IsDir()
}
