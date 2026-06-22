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
	path string
	docs []Doc
	file *os.File // held open for the writer lock (Task 6)
}

// Open returns a Collection backed by the JSONL file at path, scanning it into memory.
// Creates the file and parent dirs if absent. Supports ~ expansion.
func Open(path string) (*Collection, error) {
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
	c := &Collection{path: expanded, file: f}
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
		if p(d) {
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
		if p(d) {
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
