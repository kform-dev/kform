package fsys

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing/fstest"
)

type FS interface {
	Open(path string) (fs.File, error)

	//OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)

	//Create(path string) (*os.File, error)

	// Readfile returns the content of a given file
	ReadFile(path string) ([]byte, error)

	// WriteFile writes the data to a file at the given path,
	// it overwrites existing content
	WriteFile(path string, data []byte, perm fs.FileMode) error

	// Calculate a sha256 cheksum on the file
	Sha256(path string) (string, error)

	// Walk walks the file system with the given WalkDirFunc.
	Walk(path string, walkFn fs.WalkDirFunc) error

	// Exists is true if the path exists in the file system.
	Exists(path string) bool

	// Stat returns a FileInfo describing the named file from the file system.
	Stat(path string) (fs.FileInfo, error)

	// Glob returns the list of matching files,
	// emulating https://golang.org/pkg/path/filepath/#Glob
	Glob(pattern string) ([]string, error)
}

func NewMemFS(rootpath string, fs fstest.MapFS) FS {
	if fs == nil {
		fs = fstest.MapFS{}
	}
	return &fsys{
		kind:     "mem",
		rootPath: rootpath,
		fsys:     fs,
	}
}

func NewDiskFS(path string) FS {
	return &fsys{
		rootPath: ".",
		fsys:     os.DirFS(path),
	}
}

func NewFS(fs fs.FS) FS {
	return &fsys{
		rootPath: ".",
		fsys:     fs,
	}
}

type fsys struct {
	kind     string
	rootPath string
	fsys     fs.FS
}

func (r *fsys) Open(path string) (fs.File, error) {
	return r.fsys.Open(path)
}

func OpenFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(filepath.Join(path), flag, perm)
}

func Create(path string) (*os.File, error) {
	if filepath.Dir(path) != "" {
		os.MkdirAll(filepath.Dir(path), 0755|os.ModeDir)
	}
	return os.Create(filepath.Join(path))
}

func (r *fsys) ReadFile(path string) ([]byte, error) {
	fmt.Println("testreadfile", path)
	f, err := r.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := &bytes.Buffer{}
	buffer := make([]byte, 1024*1024)

	for {
		n, err := f.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		buf.Write(buffer[:n])
	}
	return buf.Bytes(), nil
}

func (r *fsys) Sha256(path string) (string, error) {
	f, err := r.fsys.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	buffer := make([]byte, 1024*1024)

	for {
		n, err := f.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		hash.Write(buffer[:n])
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (r *fsys) WriteFile(path string, data []byte, perm fs.FileMode) error {
	if r.kind == "mem" {
		mapFS, ok := r.fsys.(fstest.MapFS)
		if !ok {
			return fmt.Errorf("expecting mapFS, got: %s", reflect.TypeOf(r.fsys).Name())
		}
		mapFS[path] = &fstest.MapFile{
			Data: data,
		}
		r.fsys = mapFS
		return nil
	}

	if filepath.Dir(path) != "" {
		os.MkdirAll(filepath.Dir(filepath.Join(r.rootPath, path)), 0755|os.ModeDir)
	}
	return os.WriteFile(filepath.Join(r.rootPath, path), data, perm)
}

func (r *fsys) Glob(pattern string) ([]string, error) {
	return fs.Glob(r.fsys, pattern)
}

func (r *fsys) Walk(path string, walkFn fs.WalkDirFunc) error {
	return fs.WalkDir(r.fsys, path, walkFn)
}

func (r *fsys) Exists(path string) bool {
	if _, err := fs.Stat(r.fsys, path); err != nil {
		return false
	}
	return true
}

func (r *fsys) Stat(path string) (fs.FileInfo, error) {
	return fs.Stat(r.fsys, path)
}
