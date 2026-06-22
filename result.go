package jsonldb

// Result is a chainable view over a filtered set of Docs.
type Result struct {
	docs []Doc
}

func (r *Result) Docs() []Doc { return r.docs }
func (r *Result) Count() int  { return len(r.docs) }

func (r *Result) First() (Doc, bool) {
	if len(r.docs) == 0 {
		return Doc{}, false
	}
	return r.docs[0], true
}

func (r *Result) Last() (Doc, bool) {
	if len(r.docs) == 0 {
		return Doc{}, false
	}
	return r.docs[len(r.docs)-1], true
}
