package ignore

import (
	"io/fs"
	"log/slog"
)

// matcher is a function capable of computing a match.
//
// It returns true if the rule matches.
type matcher func(name string, d fs.DirEntry) bool

// pattern describes a pattern to be matched in a rule set.
type pattern struct {
	// raw is the unparsed string, with nothing stripped.
	raw string
	// match is the matcher function.
	match matcher
	// negate indicates that the rule's outcome should be negated.
	negate bool
	// mustDir indicates that the matched file must be a directory.
	mustDir bool
}

// rules is a collection of pattern matching rules.
type Rules struct {
	ignoreFile string
	patterns   []*pattern
}

// Empty builds an empty ruleset.
func Empty(ignoreFile string) *Rules {
	return &Rules{
		ignoreFile: ignoreFile,
		patterns:   []*pattern{},
	}
}

// Ignore evaluates the file at the given path, and returns true if it should be ignored.
//
// Ignore evaluates path against the rules in order. Evaluation stops when a match
// is found. Matching a negative rule will stop evaluation.
func (r *Rules) Ignore(path string, d fs.DirEntry) bool {
	// Don't match on empty dirs.
	if path == "" {
		return false
	}

	// Disallow ignoring the current working directory.
	if path == "." || path == "./" {
		return false
	}
	// ignore the ignorefile
	if path == r.ignoreFile {
		return true
	}
	for _, p := range r.patterns {
		if p.match == nil {
			slog.Info("ignore no matcher supplied", "rule", p.raw)
			return false
		}

		// For negative rules, we need to capture and return non-matches,
		// and continue for matches.
		if p.negate {
			if p.mustDir && !d.IsDir() {
				return true
			}
			if !p.match(path, d) {
				return true
			}
			continue
		}

		// If the rule is looking for directories, and this is not a directory,
		// skip it.
		if p.mustDir && !d.IsDir() {
			continue
		}
		if p.match(path, d) {
			return true
		}
	}
	return false
}
