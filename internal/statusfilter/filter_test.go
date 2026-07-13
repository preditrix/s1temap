package statusfilter

import "testing"

func TestFilter_Match(t *testing.T) {
	cases := []struct {
		query  string
		status int
		want   bool
	}{
		{"", 200, true},        // empty matches all
		{"200", 200, true},     // exact
		{"200", 404, false},    //
		{"!200", 404, true},    // negation
		{"!200", 200, false},   //
		{"500-599", 503, true}, // range
		{"500-599", 499, false},
		{"200,404", 404, true}, // OR
		{"200,404", 500, false},
		{">400", 500, true},
		{"<300", 200, true},
		{"<300", 300, false},
	}
	for _, c := range cases {
		f, err := Parse(c.query)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", c.query, err)
		}
		if got := f.Match(c.status); got != c.want {
			t.Errorf("Parse(%q).Match(%d) = %v, want %v", c.query, c.status, got, c.want)
		}
	}
}

func TestParse_Malformed(t *testing.T) {
	for _, q := range []string{"abc", "200-", "500-100", ">x"} {
		if _, err := Parse(q); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", q)
		}
	}
}
