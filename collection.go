package jsonldb

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// Collection is a file-backed JSONL store.
type Collection struct {
	path      string
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
	// Close any existing file handle and reopen fresh from disk.
	if c.file != nil {
		c.file.Close()
	}
	f, err := os.OpenFile(c.path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	c.file = f

	if err := c.buildIndex(); err != nil {
		return err
	}
	fi, err := os.Stat(c.path)
	if err != nil {
		return err
	}
	c.lazy = fi.Size() > c.threshold
	c.cache = map[int]Doc{}
	c.order = nil
	if !c.lazy {
		// eager: parse everything now; a malformed line is fatal (as before)
		for i := range c.index {
			if _, err := c.materialize(i); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Collection) All() []Doc {
	out := make([]Doc, 0, len(c.index))
	for i := range c.index {
		if d, ok := c.mustDoc(i); ok {
			out = append(out, d)
		}
	}
	return out
}

func (c *Collection) Each(fn func(Doc) bool) {
	for i := range c.index {
		d, ok := c.mustDoc(i)
		if !ok {
			continue
		}
		if !fn(d) {
			return
		}
	}
}

func (c *Collection) Count() int { return len(c.index) }

func (c *Collection) First() (Doc, bool) {
	if len(c.index) == 0 {
		return Doc{}, false
	}
	return c.mustDoc(0)
}

func (c *Collection) Last() (Doc, bool) {
	if len(c.index) == 0 {
		return Doc{}, false
	}
	return c.mustDoc(len(c.index) - 1)
}

// Where returns a Result of docs matching p.
func (c *Collection) Where(p Predicate) *Result {
	var idx []int
	for i := range c.index {
		raw, err := c.readRaw(i)
		if err != nil {
			continue
		}
		if p.rawReject(raw) {
			continue
		}
		d, ok := c.mustDoc(i)
		if ok && p.Match(d) {
			idx = append(idx, i)
		}
	}
	return &Result{col: c, idx: idx}
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
	for i := range c.index {
		raw, err := c.readRaw(i)
		if err != nil {
			continue
		}
		if p.rawReject(raw) {
			continue
		}
		if d, ok := c.mustDoc(i); ok && p.Match(d) {
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

type recordSrc struct {
	idx         int
	replacement []byte // nil ⇒ copy original bytes of index[idx]
}

func (c *Collection) rewriteRecords(srcs []recordSrc) error {
	dir := filepath.Dir(c.path)
	tmp, err := os.CreateTemp(dir, ".jsonldb-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	w := bufio.NewWriter(tmp)
	writeLine := func(b []byte) error {
		b = bytes.TrimRight(b, "\n")
		if len(bytes.TrimSpace(b)) == 0 {
			return nil
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		return w.WriteByte('\n')
	}
	for _, s := range srcs {
		if s.replacement != nil {
			if err := writeLine(s.replacement); err != nil {
				tmp.Close()
				return err
			}
			continue
		}
		raw, err := c.readRaw(s.idx)
		if err != nil {
			tmp.Close()
			return err
		}
		if err := writeLine(raw); err != nil {
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

// Append writes one doc as a JSON line at the end of the file (atomic rewrite).
func (c *Collection) Append(d Doc) error { return c.AppendAll([]Doc{d}) }

// AppendAll appends multiple docs in a single atomic rewrite.
func (c *Collection) AppendAll(ds []Doc) error {
	srcs := make([]recordSrc, 0, len(c.index)+len(ds))
	for i := range c.index {
		srcs = append(srcs, recordSrc{idx: i})
	}
	for _, d := range ds {
		b, err := d.MarshalJSON()
		if err != nil {
			return err
		}
		srcs = append(srcs, recordSrc{idx: -1, replacement: b})
	}
	return c.rewriteRecords(srcs)
}

// updateChecked applies mut to every doc matching p; if mut returns a non-nil
// error for ANY matched doc the file is NOT rewritten and (0, err) is returned.
// Otherwise it rewrites once and returns the true count.
func (c *Collection) updateChecked(p Predicate, mut func(Doc) (Doc, error)) (int, error) {
	n := 0
	srcs := make([]recordSrc, 0, len(c.index))
	for i := range c.index {
		d, ok := c.mustDoc(i)
		if ok && p.Match(d) {
			nd, err := mut(d)
			if err != nil {
				return 0, err
			}
			b, err := nd.MarshalJSON()
			if err != nil {
				return 0, err
			}
			srcs = append(srcs, recordSrc{idx: -1, replacement: b})
			n++
		} else {
			srcs = append(srcs, recordSrc{idx: i})
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, c.rewriteRecords(srcs)
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
	srcs := make([]recordSrc, 0, len(c.index))
	for i := range c.index {
		d, ok := c.mustDoc(i)
		if ok && p.Match(d) {
			n++
			continue
		}
		srcs = append(srcs, recordSrc{idx: i})
	}
	if n == 0 {
		return 0, nil
	}
	return n, c.rewriteRecords(srcs)
}

// DeleteAt removes the doc at the given 1-based scan line.
func (c *Collection) DeleteAt(line int) error {
	srcs := make([]recordSrc, 0, len(c.index))
	found := false
	for i := range c.index {
		if c.index[i].line == line {
			found = true
			continue
		}
		srcs = append(srcs, recordSrc{idx: i})
	}
	if !found {
		return nil
	}
	return c.rewriteRecords(srcs)
}

// Compact rewrites the file, dropping blank lines and exact-duplicate records.
func (c *Collection) Compact() error {
	seen := map[string]bool{}
	srcs := make([]recordSrc, 0, len(c.index))
	for i := range c.index {
		raw, err := c.readRaw(i)
		if err != nil {
			continue
		}
		key := string(raw)
		if seen[key] {
			continue
		}
		seen[key] = true
		srcs = append(srcs, recordSrc{idx: i})
	}
	return c.rewriteRecords(srcs)
}
