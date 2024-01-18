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

// ValidateDirPath validates if the filepath is a directory
func ValidateDirPath(path string) error {
	dir, _ := filepath.Split(path)
	if dir == "" {
		return ErrPathNoDirectory
	}
	return nil
}

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
