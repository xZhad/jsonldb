package jsonldb

import (
	"bufio"
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
