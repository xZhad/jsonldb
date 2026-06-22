package jsonldb

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

type meta struct {
	offset int64
	length int
	line   int // 1-based physical line
}

// buildIndex scans newline offsets without parsing. Blank lines are skipped.
func (c *Collection) buildIndex() error {
	f, err := os.Open(c.path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<20)
	var index []meta
	var off int64
	lineNo := 0
	for {
		lineNo++
		lineBytes, err := r.ReadBytes('\n')
		n := int64(len(lineBytes))
		content := bytes.TrimRight(lineBytes, "\n")
		// trim the trailing \r if present (CRLF), but keep length aligned to bytes read
		trimmed := bytes.TrimRight(content, "\r")
		if len(bytes.TrimSpace(trimmed)) > 0 {
			index = append(index, meta{offset: off, length: len(trimmed), line: lineNo})
		}
		off += n
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	c.index = index
	return nil
}

// readRaw returns the raw bytes of record i (from cache or via ReadAt).
func (c *Collection) readRaw(i int) ([]byte, error) {
	if d, ok := c.cache[i]; ok {
		return d.Raw(), nil
	}
	m := c.index[i]
	buf := make([]byte, m.length)
	if _, err := c.file.ReadAt(buf, m.offset); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

// materialize returns the parsed Doc for record i, caching it.
func (c *Collection) materialize(i int) (Doc, error) {
	if d, ok := c.cache[i]; ok {
		c.touch(i)
		return d, nil
	}
	raw, err := c.readRaw(i)
	if err != nil {
		return Doc{}, err
	}
	d, err := parseDoc(raw, c.index[i].line)
	if err != nil {
		return Doc{}, err
	}
	c.put(i, d)
	return d, nil
}

// mustDoc materializes record i, returning ok=false on error (skip).
func (c *Collection) mustDoc(i int) (Doc, bool) {
	d, err := c.materialize(i)
	if err != nil {
		return Doc{}, false
	}
	return d, true
}

func (c *Collection) put(i int, d Doc) {
	if c.cache == nil {
		c.cache = map[int]Doc{}
	}
	if _, exists := c.cache[i]; !exists {
		c.order = append(c.order, i)
	}
	c.cache[i] = d
	if c.lazy && c.cacheCap > 0 {
		for len(c.cache) > c.cacheCap && len(c.order) > 0 {
			old := c.order[0]
			c.order = c.order[1:]
			delete(c.cache, old)
		}
	}
}

func (c *Collection) touch(i int) {
	if !c.lazy {
		return
	}
	for j, v := range c.order {
		if v == i {
			c.order = append(c.order[:j], c.order[j+1:]...)
			c.order = append(c.order, i)
			break
		}
	}
}
