package jsonldb

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Collection is a file-backed JSONL store, loaded fully into memory.
type Collection struct {
	path      string
	docs      []Doc
	file      *os.File // held open for the writer lock (Task 6)
	threshold int64
	cacheCap  int
	index     []meta
	cache     map[int]Doc
	order     []int
	lazy      bool
}

// Option configures a Collection at Open time.
type Option func(*Collection)

// WithEagerThreshold sets the file-size cutoff (bytes) below which the whole
// file is parsed eagerly at Open; larger files use lazy/streaming access.
func WithEagerThreshold(bytes int64) Option { return func(c *Collection) { c.threshold = bytes } }

// WithCacheSize sets the lazy-mode materialized-Doc LRU capacity.
func WithCacheSize(n int) Option { return func(c *Collection) { c.cacheCap = n } }

// Open returns a Collection backed by the JSONL file at path, scanning it into memory.
// Creates the file and parent dirs if absent. Supports ~ expansion.
func Open(path string, opts ...Option) (*Collection, error) {
	expanded, err := expandPath(path)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(expanded, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	c := &Collection{path: expanded, file: f, threshold: 8 << 20, cacheCap: 4096, cache: map[int]Doc{}, order: []int{}}
	for _, opt := range opts {
		opt(c)
	}
	if err := c.scan(); err != nil {
		f.Close()
		return nil, err
	}
	return c, nil
}

func (c *Collection) Path() string { return c.path }

// Reload re-scans the file from disk.
func (c *Collection) Reload() error { return c.scan() }

// Close releases the file handle (writer lock).
func (c *Collection) Close() error {
	if c.file != nil {
		err := c.file.Close()
		c.file = nil
		return err
	}
	return nil
}

func (c *Collection) scan() error {
	f, err := os.Open(c.path)
	if err != nil {
		return err
	}
	defer f.Close()
	var docs []Doc
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		d, err := parseDoc(line, lineNo)
		if err != nil {
			return err
		}
		docs = append(docs, d)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	c.docs = docs
	if err := c.buildIndex(); err != nil {
		return err
	}
	c.cache = nil
	c.order = nil
	return nil
}

func (c *Collection) All() []Doc { return c.docs }

func (c *Collection) Each(fn func(Doc) bool) {
	for _, d := range c.docs {
		if !fn(d) {
			return
		}
	}
}

func (c *Collection) Count() int { return len(c.docs) }

func (c *Collection) First() (Doc, bool) {
	if len(c.docs) == 0 {
		return Doc{}, false
	}
	return c.docs[0], true
}

func (c *Collection) Last() (Doc, bool) {
	if len(c.docs) == 0 {
		return Doc{}, false
	}
	return c.docs[len(c.docs)-1], true
}

// Where returns a Result of docs matching p.
func (c *Collection) Where(p Predicate) *Result {
	var out []Doc
	for _, d := range c.docs {
		if p.Match(d) {
			out = append(out, d)
		}
	}
	return &Result{docs: out}
}

// Query parses a DSL string and returns the matching Result.
func (c *Collection) Query(dsl string) (*Result, error) {
	p, err := parseDSL(dsl)
	if err != nil {
		return nil, err
	}
	return c.Where(p), nil
}

// Find returns the first doc matching p.
func (c *Collection) Find(p Predicate) (Doc, bool) {
	for _, d := range c.docs {
		if p.Match(d) {
			return d, true
		}
	}
	return Doc{}, false
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path, err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return filepath.Abs(path)
}

// Append writes one doc as a JSON line at the end of the file (atomic rewrite).
func (c *Collection) Append(d Doc) error {
	return c.AppendAll([]Doc{d})
}

// AppendAll appends multiple docs in a single atomic rewrite.
func (c *Collection) AppendAll(ds []Doc) error {
	lines := make([][]byte, 0, len(c.docs)+len(ds))
	for _, d := range c.docs {
		lines = append(lines, d.Raw())
	}
	for _, d := range ds {
		b, err := d.MarshalJSON()
		if err != nil {
			return err
		}
		lines = append(lines, b)
	}
	return c.rewrite(lines)
}

// rewrite writes lines atomically (temp file + rename), then re-scans.
func (c *Collection) rewrite(lines [][]byte) error {
	dir := filepath.Dir(c.path)
	tmp, err := os.CreateTemp(dir, ".jsonldb-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	w := bufio.NewWriter(tmp)
	for _, ln := range lines {
		ln = bytes.TrimRight(ln, "\n")
		if len(bytes.TrimSpace(ln)) == 0 {
			continue
		}
		if _, err := w.Write(ln); err != nil {
			tmp.Close()
			return err
		}
		if err := w.WriteByte('\n'); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		return err
	}
	return c.scan()
}

// updateChecked applies mut to every doc matching p; if mut returns a non-nil
// error for ANY matched doc the file is NOT rewritten and (0, err) is returned.
// Otherwise it rewrites once and returns the true count.
func (c *Collection) updateChecked(p Predicate, mut func(Doc) (Doc, error)) (int, error) {
	n := 0
	lines := make([][]byte, 0, len(c.docs))
	for _, d := range c.docs {
		if p.Match(d) {
			nd, err := mut(d)
			if err != nil {
				return 0, err
			}
			b, err := nd.MarshalJSON()
			if err != nil {
				return 0, err
			}
			lines = append(lines, b)
			n++
		} else {
			lines = append(lines, d.Raw())
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, c.rewrite(lines)
}

// Update applies mut to every doc matching p; returns the count changed.
func (c *Collection) Update(p Predicate, mut func(Doc) Doc) (int, error) {
	return c.updateChecked(p, func(d Doc) (Doc, error) { return mut(d), nil })
}

// Replace swaps every doc matching p with d.
func (c *Collection) Replace(p Predicate, d Doc) (int, error) {
	return c.Update(p, func(Doc) Doc { return d })
}

// DeleteWhere removes every doc matching p; returns the count removed.
func (c *Collection) DeleteWhere(p Predicate) (int, error) {
	n := 0
	lines := make([][]byte, 0, len(c.docs))
	for _, d := range c.docs {
		if p.Match(d) {
			n++
			continue
		}
		lines = append(lines, d.Raw())
	}
	if n == 0 {
		return 0, nil
	}
	return n, c.rewrite(lines)
}

// DeleteAt removes the doc at the given 1-based scan line.
func (c *Collection) DeleteAt(line int) error {
	lines := make([][]byte, 0, len(c.docs))
	found := false
	for _, d := range c.docs {
		if d.Line() == line {
			found = true
			continue
		}
		lines = append(lines, d.Raw())
	}
	if !found {
		return nil
	}
	return c.rewrite(lines)
}

// Compact rewrites the file, dropping blank lines and exact-duplicate records.
func (c *Collection) Compact() error {
	seen := map[string]bool{}
	lines := make([][]byte, 0, len(c.docs))
	for _, d := range c.docs {
		key := string(d.Raw())
		if seen[key] {
			continue
		}
		seen[key] = true
		lines = append(lines, d.Raw())
	}
	return c.rewrite(lines)
}
