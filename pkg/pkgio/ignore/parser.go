package ignore

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
)

// Parse parses a rules file and return empty rules
func Parse(file io.Reader) (*Rules, error) {
	r := &Rules{patterns: []*pattern{}}

	s := bufio.NewScanner(file)
	currentLine := 0
	utf8bom := []byte{0xEF, 0xBB, 0xBF}
	for s.Scan() {
		scannedBytes := s.Bytes()
		// We trim UTF8 BOM
		if currentLine == 0 {
			scannedBytes = bytes.TrimPrefix(scannedBytes, utf8bom)
		}
		line := string(scannedBytes)
		currentLine++

		if err := r.parseRule(line); err != nil {
			return r, err
		}
	}
	return r, s.Err()
}

// parseRule parses a rule string and creates a pattern, which is then stored in the Rules object.
func (r *Rules) parseRule(rule string) error {
	rule = strings.TrimSpace(rule)

	// Ignore blank lines
	if rule == "" {
		return nil
	}
	// Comment
	if strings.HasPrefix(rule, "#") {
		return nil
	}

	// Fail any rules that contain **
	if strings.Contains(rule, "**") {
		return errors.New("double-star (**) syntax is not supported")
	}

	// Fail any patterns that can't compile. A non-empty string must be
	// given to Match() to avoid optimization that skips rule evaluation.
	if _, err := filepath.Match(rule, "abc"); err != nil {
		return err
	}

	p := &pattern{raw: rule}

	// Negation is handled at a higher level, so strip the leading ! from the
	// string.
	if strings.HasPrefix(rule, "!") {
		p.negate = true
		rule = rule[1:]
	}

	// Directory verification is handled by a higher level, so the trailing /
	// is removed from the rule. That way, a directory named "foo" matches,
	// even if the supplied string does not contain a literal slash character.
	if strings.HasSuffix(rule, "/") {
		p.mustDir = true
		rule = strings.TrimSuffix(rule, "/")
	}

	if strings.HasPrefix(rule, "/") {
		// Require path matches the root path.
		p.match = func(n string, d fs.DirEntry) bool {
			rule = strings.TrimPrefix(rule, "/")
			ok, err := filepath.Match(rule, n)
			if err != nil {
				log.Printf("Failed to compile %q: %s", rule, err)
				return false
			}
			return ok
		}
	} else if strings.Contains(rule, "/") {
		// require structural match.
		p.match = func(n string, d fs.DirEntry) bool {
			ok, err := filepath.Match(rule, n)
			if err != nil {
				log.Printf("Failed to compile %q: %s", rule, err)
				return false
			}
			return ok
		}
	} else {
		p.match = func(n string, d fs.DirEntry) bool {
			// When there is no slash in the pattern, we evaluate ONLY the
			// filename.
			n = filepath.Base(n)
			ok, err := filepath.Match(rule, n)
			if err != nil {
				log.Printf("Failed to compile %q: %s", rule, err)
				return false
			}
			return ok
		}
	}

	r.patterns = append(r.patterns, p)
	return nil
}
