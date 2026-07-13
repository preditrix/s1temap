// Package statusfilter parses a compact status-code query into a matcher.
//
// Grammar (comma-separated terms, OR-ed together; empty query matches all):
//
//	200        exact
//	200-299    inclusive range
//	>400       strictly greater than
//	<300       strictly less than
//	!200       negation of any term (recursive), e.g. !200-299
//	200,404    OR of terms
package statusfilter

import (
	"fmt"
	"strconv"
	"strings"
)

// Filter is an immutable, concurrency-safe status matcher compiled once from a
// query string.
type Filter struct {
	preds []func(int) bool
}

// Parse compiles query into a Filter. An empty query yields a Filter that
// matches every status. A malformed term returns an error.
func Parse(query string) (*Filter, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return &Filter{}, nil
	}

	var preds []func(int) bool
	for _, term := range strings.Split(query, ",") {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		pred, err := parseTerm(term)
		if err != nil {
			return nil, err
		}
		preds = append(preds, pred)
	}
	return &Filter{preds: preds}, nil
}

// Match reports whether status satisfies the filter. An empty filter matches
// everything.
func (f *Filter) Match(status int) bool {
	if len(f.preds) == 0 {
		return true
	}
	for _, pred := range f.preds {
		if pred(status) {
			return true
		}
	}
	return false
}

func parseTerm(term string) (func(int) bool, error) {
	switch {
	case strings.HasPrefix(term, "!"):
		inner, err := parseTerm(strings.TrimSpace(term[1:]))
		if err != nil {
			return nil, err
		}
		return func(s int) bool { return !inner(s) }, nil

	case strings.HasPrefix(term, ">"):
		n, err := parseInt(term[1:])
		if err != nil {
			return nil, err
		}
		return func(s int) bool { return s > n }, nil

	case strings.HasPrefix(term, "<"):
		n, err := parseInt(term[1:])
		if err != nil {
			return nil, err
		}
		return func(s int) bool { return s < n }, nil

	case strings.Contains(term, "-"):
		parts := strings.SplitN(term, "-", 2)
		lo, err := parseInt(parts[0])
		if err != nil {
			return nil, err
		}
		hi, err := parseInt(parts[1])
		if err != nil {
			return nil, err
		}
		if lo > hi {
			return nil, fmt.Errorf("invalid status range %q: %d > %d", term, lo, hi)
		}
		return func(s int) bool { return s >= lo && s <= hi }, nil

	default:
		n, err := parseInt(term)
		if err != nil {
			return nil, err
		}
		return func(s int) bool { return s == n }, nil
	}
}

func parseInt(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid status filter term %q: %w", s, err)
	}
	return n, nil
}
