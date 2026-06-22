package jsonldb

import (
	"path/filepath"
	"testing"
)

func TestOpenOptions(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.jsonl")
	c, err := Open(p, WithEagerThreshold(0), WithCacheSize(16))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()
	if c.threshold != 0 || c.cacheCap != 16 {
		t.Errorf("opts not applied: threshold=%d cacheCap=%d", c.threshold, c.cacheCap)
	}
}

func TestOpenDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "y.jsonl")
	c, _ := Open(p)
	defer c.Close()
	if c.threshold != 8<<20 || c.cacheCap != 4096 {
		t.Errorf("defaults wrong: threshold=%d cacheCap=%d", c.threshold, c.cacheCap)
	}
}
