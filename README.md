# jsonldb

A tiny, zero-dependency Go library for JSONL collections. **Dynamic at the core, typed as sugar.**

The on-disk reality of JSONL is untyped text — one JSON object per line. `jsonldb` embraces that: the base layer is a dynamic `Doc` (`map[string]any`) with a real query engine (string DSL + builder), structure discovery, aggregation, and sort/page. A thin generic `Typed[T]` layer sits on top for callers who want structs.

## Features

- **Zero dependencies** — stdlib only (`encoding/json`, `os`, `bufio`, `regexp`, `strconv`, `time`)
- **Dynamic core, typed sugar** — the file is untyped text; structs are an opt-in view, not the foundation
- **The file is the source of truth** — `grep`/`jq`-friendly on disk; lossless rewrites preserve untouched lines verbatim
- **One engine** — query DSL, coercion, discovery, aggregation, and sort/page reused by every consumer
- **Full CRUD, atomically** — append/update/replace/delete via temp-file + `fsync` + `rename`
- **In-memory querying** — load once, filter in Go; JSONL files are small by design

## Install

```sh
go get github.com/xZhad/jsonldb
```

Requires Go 1.26+.

## Quick start

```go
package main

import (
	"fmt"

	"github.com/xZhad/jsonldb"
)

func main() {
	c, err := jsonldb.Open("sessions.jsonl") // creates the file + parent dirs if absent
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// Append a record.
	c.Append(jsonldb.NewDoc(map[string]any{
		"id":        "s_1",
		"topic":     "axiom ch3",
		"completed": true,
	}))

	// Query with the string DSL.
	res, err := c.Query("completed=true topic~=axiom")
	if err != nil {
		panic(err) // DSL parse errors are returned, never panic the program
	}
	for _, d := range res.Docs() {
		fmt.Println(d.GetString("topic"))
	}
}
```

## The `Doc` model

`Doc` is the dynamic record — a value-bag plus the metadata needed for lossless rewrites and line-addressed deletes.

```go
d := jsonldb.NewDoc(map[string]any{"topic": "ML", "duration": 1500})

d.Get("topic")            // (any, bool)
d.GetString("topic")      // "ML"
d.GetInt("duration")      // (1500, true)
d.GetTime("started")      // (time.Time, bool) — RFC3339
d.Path("notes.0.text")    // nested access via dotted keys + numeric indices
d.Raw()                   // original line bytes, verbatim
d.Line()                  // stable 1-based scan index
```

Numbers decode as `json.Number` (via `UseNumber()`), so ints stay ints on rewrite and numeric comparison never loses precision. `Doc` marshals its fields in **stable (sorted) key order**, giving rewritten records a deterministic layout.

## Collection API

```go
// lifecycle
jsonldb.Open(path)   // creates file + parent dirs if absent; ~ expansion
c.Reload()           // re-scan from disk
c.Close()
c.Path()

// read
c.All()              // []Doc
c.Each(func(Doc) bool) // stream; return false to stop
c.Count()
c.First() / c.Last()
c.Find(pred)         // first match

// query — both return *Result
c.Query("completed=true topic~=jsonl")
c.Where(jsonldb.Eq("completed", true))

// mutation — atomic temp+rename, lossless for untouched lines
c.Append(d)
c.AppendAll(ds)                          // one rewrite
c.Update(pred, func(Doc) Doc)            // (n, err)
c.Replace(pred, d)
c.DeleteWhere(pred)
c.DeleteAt(line)                         // delete by Doc.Line()
c.Compact()                              // drop blank lines / dedup

// structure discovery
c.Schema()           // per-key: types seen, presence %, sample
c.Keys()             // union of all keys
c.Sample(n)
```

### Builder predicates

```go
jsonldb.Eq(k, v)      jsonldb.Ne(k, v)
jsonldb.Gt(k, v)      jsonldb.Gte(k, v)
jsonldb.Lt(k, v)      jsonldb.Lte(k, v)
jsonldb.Contains(k, substr)   // case-insensitive
jsonldb.HasKey(k)
jsonldb.Prefix(k, p)  jsonldb.Suffix(k, s)
jsonldb.Regex(k, pattern)
jsonldb.In(k, vs...)  jsonldb.Between(k, lo, hi)

jsonldb.And(...)  jsonldb.Or(...)  jsonldb.Not(...)
```

### Aggregation, sort & page (on `*Result`)

```go
res := c.Where(jsonldb.Eq("completed", true))

res.GroupBy("topic")                 // map[string]*Result
res.GroupByFunc(func(Doc) string {…}) // computed bucket (e.g. day from timestamp)
res.CountBy("topic")
res.Distinct("topic")
res.Sum("duration")  // also Avg / Min / Max

res.SortBy("duration", true).Page(1, 20).Docs()
```

## Query DSL

A recursive-descent grammar with grouping and four precedence levels (`|=` < space-AND < `!` < `()`):

```
completed=true topic~=jsonl       # AND
a=1 (b=2 |= c=3)                  # a AND (b OR c)
!completed topic~=jsonl           # (NOT completed) AND contains
!(status=done |= archived)        # grouped NOT
duration>=1500 started>=2026-06-01 # numeric + RFC3339 time compare
notes                             # bare key = has-key
title=~^WIP                       # regex
```

| DSL | Meaning | Builder |
|-----|---------|---------|
| `=` | equal (coerced) | `Eq` |
| `!=` | not equal | `Ne` |
| `>` `>=` `<` `<=` | ordered compare | `Gt`/`Gte`/`Lt`/`Lte` |
| `~=` | substring, case-insensitive | `Contains` |
| `^=` | prefix | `Prefix` |
| `$=` | suffix | `Suffix` |
| `=~` | regex | `Regex` |
| `key` (bare) | exists | `HasKey` |

Values type automatically: `true`/`false`/`null`, bare numbers → `json.Number`, RFC3339 → time, `"quoted"` → forced string, else bare string.

## Typed layer

A thin generic wrapper over `*Collection` that decodes `Doc`↔`T` via `encoding/json`, with no new storage logic.

```go
type Session struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	Completed bool   `json:"completed"`
}

t := jsonldb.Typed[Session](c)

t.Append(Session{ID: "s_1", Topic: "axiom", Completed: true})
sessions, _ := t.Query("completed=true")        // []Session
t.Update(jsonldb.Eq("id", "s_1"), func(s Session) Session {
	s.Completed = false
	return s
})

t.Raw() // drop back to the dynamic *Collection anytime
```

Predicates stay string-keyed (they operate on the wire shape), so the same queries work typed or dynamic. Filtering runs on the cheap `Doc` layer first; only survivors decode to `T`. Aggregation stays on the dynamic `*Result` — reach it via `t.Where(p).Raw().GroupBy(...)`.

> **Note:** `Typed.Update` round-trips through `T`, so fields absent from your struct are **dropped** on re-encode. For lossless edits over arbitrary JSONL, use the core `Update(func(Doc) Doc)`.

## File format

Standard JSONL — one JSON object per line, UTF-8, newline-terminated. Fully compatible with `jq`, `grep`, `wc -l`, and any other JSONL tooling.

```
{"id":"s_1","topic":"axiom ch3","started":"2026-06-08T18:30:00-04:00","completed":true}
{"id":"s_2","topic":"ML theory","started":"2026-06-08T19:00:00-04:00","completed":false}
```

## Safety & atomicity

- **Atomic mutations** — write a temp file in the same dir, `fsync`, then `rename` over the original. A crash leaves either the old file or the new, never a half-written one.
- **Lossless** — untouched records are copied byte-for-byte from their original lines; only the changed record is re-serialized.
- **Single-writer** — `Open` holds the backing file handle open until `Close`, but does **not** take an OS advisory lock (`flock`). This library assumes single-process use; do not rely on a second writer being rejected.

### Sharp edges

- **`Doc.Line()` is valid only against the current scan generation.** Every mutation rewrites the file and re-scans, renumbering lines densely. Read a `Line()` value and act on it within the same scan generation.
- **Read slices are not snapshots.** `All()`, `Sample(n)`, and `Result.Limit`/`Offset` return slices backed by internal storage — do not `append` into them. `SortBy` returns a fresh copy.

## Testing

```sh
go test ./...
```
