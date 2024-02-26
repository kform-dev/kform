package pkgio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/henderiw/store"
	"github.com/henderiw/store/memory"
	"github.com/kform-dev/kform/pkg/fsys"
	"github.com/kform-dev/kform/pkg/pkgio/ignore"
	"github.com/pkg/errors"
)

func NewData() store.Storer[[]byte] {
	return memory.NewStore[[]byte]()
}

type PkgReader struct {
	PathExists     bool
	Fsys           fsys.FS
	MatchFilesGlob []string
	IgnoreRules    *ignore.Rules
	SkipDir        bool
	Checksum       bool
}

func (r *PkgReader) Read(ctx context.Context, data store.Storer[[]byte]) (store.Storer[[]byte], error) {
	if !r.PathExists {
		return data, nil
	}
	paths, err := r.getPaths(ctx)
	if err != nil {
		return data, err
	}
	return r.readFileContent(ctx, paths, data)
}

func (r *PkgReader) getPaths(ctx context.Context) ([]string, error) {
	// collect the paths
	paths := []string{}
	if err := r.Fsys.Walk(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Directory-based ignore rules involve skipping the entire
			// contents of that directory.
			if r.IgnoreRules.Ignore(path, d) {
				return filepath.SkipDir
			}
			if r.SkipDir && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if r.IgnoreRules.Ignore(path, d) {
			return nil
		}
		// process glob
		if match, err := r.shouldSkipFile(path); err != nil {
			return err
		} else if match {
			// skip the file
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, err
	}
	return paths, nil
}

func (r *PkgReader) readFileContent(ctx context.Context, paths []string, data store.Storer[[]byte]) (store.Storer[[]byte], error) {
	var wg sync.WaitGroup
	for _, path := range paths {
		path := path
		wg.Add(1)
		var err error
		go func() {
			defer wg.Done()
			var d []byte
			if r.Checksum {
				hash, err := r.Fsys.Sha256(path)
				if err != nil {
					return
				}
				d = []byte(hash)
				data.Create(ctx, store.ToKey(path), d)
				return
			}

			if isYamlMatch(r.MatchFilesGlob) {
				f, err := r.Fsys.Open(path)
				if err != nil {
					return
				}
				defer f.Close()
				input := &bytes.Buffer{}
				_, err = io.Copy(input, f)
				if err != nil {
					return
				}
				// Replace the ending \r\n (line ending used in windows) with \n and then split it into multiple YAML documents
				// if it contains document separators (---)
				values, err := splitDocuments(strings.ReplaceAll(input.String(), "\r\n", "\n"))
				if err != nil {
					return
				}
				for i := range values {
					// the Split used above will eat the tail '\n' from each resource. This may affect the
					// literal string value since '\n' is meaningful in it.
					if i != len(values)-1 {
						values[i] += "\n"
					}
					data.Create(ctx, store.ToKey(fmt.Sprintf("%s.%d", path, i)), []byte(values[i]))
				}
			} else {
				d, err = r.Fsys.ReadFile(path)
				if err != nil {
					return
				}
				data.Create(ctx, store.ToKey(path), d)
			}

		}()
		if err != nil {
			return nil, err
		}
	}
	wg.Wait()

	return data, nil
}

func (r *PkgReader) Write(store.Storer[[]byte]) error {
	return nil
}

func (r *PkgReader) shouldSkipFile(path string) (bool, error) {
	for _, g := range r.MatchFilesGlob {
		if match, err := filepath.Match(g, filepath.Base(path)); err != nil {
			// if err we should skip the file
			return true, err
		} else if match {
			// if matchw e should include the file
			return false, nil
		}
	}
	// if no match we should skip the file
	return true, nil
}

// splitDocuments returns a slice of all documents contained in a YAML string. Multiple documents can be divided by the
// YAML document separator (---). It allows for white space and comments to be after the separator on the same line,
// but will return an error if anything else is on the line.
func splitDocuments(s string) ([]string, error) {
	docs := make([]string, 0)
	if len(s) > 0 {
		// The YAML document separator is any line that starts with ---
		yamlSeparatorRegexp := regexp.MustCompile(`\n---.*\n`)

		// Find all separators, check them for invalid content, and append each document to docs
		separatorLocations := yamlSeparatorRegexp.FindAllStringIndex(s, -1)
		prev := 0
		for i := range separatorLocations {
			loc := separatorLocations[i]
			separator := s[loc[0]:loc[1]]

			// If the next non-whitespace character on the line following the separator is not a comment, return an error
			trimmedContentAfterSeparator := strings.TrimSpace(separator[4:])
			if len(trimmedContentAfterSeparator) > 0 && trimmedContentAfterSeparator[0] != '#' {
				return nil, errors.Errorf("invalid document separator: %s", strings.TrimSpace(separator))
			}

			docs = append(docs, s[prev:loc[0]])
			prev = loc[1]
		}
		docs = append(docs, s[prev:])
	}

	return docs, nil
}
