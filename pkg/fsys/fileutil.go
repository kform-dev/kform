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

//const tmpKformDirPrefix = "kform-diff"

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

// IsDir returns true if path represents a directory in the fileSystem
// otherwise false is returned
func IsDir(path string) bool {
	if f, err := os.Stat(path); err == nil {
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

// Directory creates a new temp directory, in which files get created.
type Directory struct {
	Path string
}

// CreateDirectory does create the actual disk directory, and return a
// new representation of it.
func CreateTempDirectory(prefix string) (*Directory, error) {
	path, err := os.MkdirTemp("", prefix+"-")
	if err != nil {
		return nil, err
	}

	return &Directory{
		Path: path,
	}, nil
}

// NewFile creates a new file in the directory.
func (r *Directory) NewFile(name string) (*os.File, error) {
	return os.OpenFile(filepath.Join(r.Path, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0700)
}

// Delete removes the directory recursively.
func (r *Directory) Delete() error {
	return os.RemoveAll(r.Path)
}

// Delete removes the directory recursively.
func (r *Directory) Name() string {
	return r.Path
}
