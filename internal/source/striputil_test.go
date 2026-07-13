package source

import (
	"reflect"
	"testing"
)

func TestStripBaseURLs(t *testing.T) {
	in := []string{
		"https://example.com/a?x=1#frag",
		"https://example.com/b",
	}
	want := []string{"/a?x=1#frag", "/b"}
	if got := StripBaseURLs(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("StripBaseURLs = %v, want %v", got, want)
	}
}
