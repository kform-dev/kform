package fsys

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

var filesysEmptyBuilders = map[string]func() FS{
	"dsk": makeEmptyDiskFs,
	"mem": makeEmptyMemFs,
}

// makeEmptyFsOnDisk makes an instance of NewDiskFS.
func makeEmptyDiskFs() FS {
	path, _ := os.MkdirTemp("", "test")
	return NewDiskFS(path)
}

// makeEmptyFsOnDisk makes an instance of NewDiskFS.
func makeEmptyMemFs() FS {
	return NewMemFS("", nil)
}

var filesysInitialBuilders = map[string]func(fstest.MapFS) FS{
	"dsk": makeInitialDiskFs,
	"mem": makeInitialMemFs,
}

// makeEmptyFsOnDisk makes an instance of NewDiskFS.
func makeInitialDiskFs(td fstest.MapFS) FS {
	path, _ := os.MkdirTemp("", "test")
	fsys := NewDiskFS(path)
	for path, mapFile := range td {
		if err := fsys.WriteFile(path, mapFile.Data, 0644); err != nil {
			fmt.Println("err", err)
		}
	}
	return fsys
}

// makeEmptyFsOnDisk makes an instance of NewDiskFS.
func makeInitialMemFs(td fstest.MapFS) FS {
	return NewMemFS("", td)
}

func TestNotExistErr(t *testing.T) {
	for name, builder := range filesysEmptyBuilders {
		t.Run(name, func(t *testing.T) {
			testNotExistErr(t, builder())
		})
	}
}

func testNotExistErr(t *testing.T, tfs FS) {
	t.Helper()
	const path = "bad-dir/bad-file.txt"

	err := tfs.RemoveAll(path)
	assert.Falsef(t, errors.Is(err, os.ErrNotExist), "RemoveAll should not return ErrNotExist, got %v", err)
	_, err = tfs.ReadFile(path)
	assert.Truef(t, errors.Is(err, os.ErrNotExist), "ReadFile should return ErrNotExist, got %v", err)
	err = tfs.Walk(path, func(_ string, _ fs.DirEntry, err error) error { return err })
	assert.Truef(t, errors.Is(err, os.ErrNotExist), "Walk should return ErrNotExist, got %v", err)
	exists := tfs.Exists(path)
	if exists != false {
		t.Errorf("want %t, got: %t", false, exists)
	}
}

func TestReadFS(t *testing.T) {
	for name, builder := range filesysInitialBuilders {
		data := fstest.MapFS{
			"testdata/foo/1.go":       {Data: []byte("package foo\n")},
			"testdata/foo/1/1.txt":    {Data: []byte("1111\n")},
			"testdata/foo/2/2.txt":    {Data: []byte("2222\n")},
			"testdata/foo/2/2.go":     {Data: []byte("package bar\n")},
			"testdata/foo/bar/3/3.go": {Data: []byte("package zoo\n")},
			"testdata/foo/bar/4.go":   {Data: []byte("package zoo1\n")},
		}
		t.Run(name, func(t *testing.T) {
			testReadFS(t, data, builder(data))
		})
	}
}

func testReadFS(t *testing.T, data fstest.MapFS, tfsys FS) {
	t.Helper()

	sorteddata := make([]string, 0, len(data))
	for path := range data {
		sorteddata = append(sorteddata, path)
	}
	sort.Strings(sorteddata)

	for path, mapFile := range data {
		// test readFile
		d, err := tfsys.ReadFile(path)
		assert.NoError(t, err)
		if diff := cmp.Diff(mapFile.Data, d); diff != "" {
			t.Errorf("want %s, got: %s", string(mapFile.Data), string(d))
		}
		// test Exist
		exists := tfsys.Exists(path)
		if diff := cmp.Diff(true, exists); diff != "" {
			t.Errorf("want %t, got: %t", true, exists)
		}
		// test walk
		paths := []string{}
		err = tfsys.Walk(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			paths = append(paths, path)
			return nil
		})
		assert.NoError(t, err)
		sort.Strings(paths)
		if diff := cmp.Diff(sorteddata, paths); diff != "" {
			t.Errorf("want %v, got: %v", sorteddata, paths)
		}
	}
}

// TODO add glob tests
