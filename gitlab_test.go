package main

import (
	"testing"

	"github.com/actions-precompiled/foundation"
)

func TestParseGitLabTagNames(t *testing.T) {
	t.Parallel()
	got, err := parseGitLabTagNames(`[{"name":"v1.4.43"},{"name":"v1.4.42"},{"name":""}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "v1.4.43" || got[1] != "v1.4.42" {
		t.Fatalf("got %v", got)
	}
}

func TestParseGitLabLatestRelease(t *testing.T) {
	t.Parallel()
	got, err := parseGitLabLatestRelease(`[{"tag_name":"v1.4.43","name":"v1.4.43"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.4.43" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterRemminaTags(t *testing.T) {
	t.Parallel()
	in := []string{"v1.4.43", "v1.4.42", "latest", "master", "git-conversion-1", "v1.2.0-rcgit.29", "1.4.41"}
	got := filterRemminaTags(in)
	want := map[string]bool{"v1.4.43": true, "v1.4.42": true, "v1.2.0-rcgit.29": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, tname := range got {
		if !want[tname] {
			t.Fatalf("unexpected %q in %v", tname, got)
		}
	}
	if !foundation.IsPublishableTag("v1.4.43") {
		t.Fatal("v1.4.43 should be publishable")
	}
}
