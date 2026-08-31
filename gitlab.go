package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/actions-precompiled/foundation"
)

const gitlabRemminaTagsURL = "https://gitlab.com/api/v4/projects/Remmina%2FRemmina/repository/tags?per_page=100"
const gitlabRemminaReleasesURL = "https://gitlab.com/api/v4/projects/Remmina%2FRemmina/releases?per_page=1"

// gitlabTags lists Remmina versions from GitLab and publishes to this GitHub repo.
type gitlabTags struct {
	inner foundation.GitHub
	deps  foundation.Deps
}

func newGitLabTags(deps foundation.Deps) foundation.GitHub {
	return gitlabTags{inner: deps.GitHub, deps: deps}
}

func (g gitlabTags) ListUpstreamTags(ctx context.Context, _ string) ([]string, error) {
	raw, err := g.get(ctx, gitlabRemminaTagsURL)
	if err != nil {
		return nil, err
	}
	tags, err := parseGitLabTagNames(raw)
	if err != nil {
		return nil, err
	}
	return filterRemminaTags(tags), nil
}

func (g gitlabTags) LatestReleaseTag(ctx context.Context, _ string) (string, error) {
	raw, err := g.get(ctx, gitlabRemminaReleasesURL)
	if err == nil {
		if tag, err := parseGitLabLatestRelease(raw); err == nil && tag != "" {
			return tag, nil
		}
	}
	tags, err := g.ListUpstreamTags(ctx, "")
	if err != nil {
		return "", err
	}
	tags = foundation.SortVersionStrings(tags)
	if len(tags) == 0 {
		return "", fmt.Errorf("%w", foundation.ErrNoUpstreamTags)
	}
	return tags[len(tags)-1], nil
}

func (g gitlabTags) ListReleasedTags(ctx context.Context) ([]string, error) {
	return g.inner.ListReleasedTags(ctx)
}

func (g gitlabTags) CreateRelease(ctx context.Context, req foundation.ReleaseRequest) error {
	return g.inner.CreateRelease(ctx, req)
}

func (g gitlabTags) DeleteRelease(ctx context.Context, tag string) error {
	return g.inner.DeleteRelease(ctx, tag)
}

func (g gitlabTags) get(ctx context.Context, url string) (string, error) {
	out, err := g.deps.Runner.Output(ctx, "curl",
		"--fail", "--silent", "--show-error", "--location",
		"--max-time", "60",
		"-H", "Accept: application/json",
		url,
	)
	if err != nil {
		return "", fmt.Errorf("gitlab %s: %w", url, err)
	}
	return out, nil
}

func parseGitLabTagNames(raw string) ([]string, error) {
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("gitlab tags json: %w", err)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if it.Name != "" {
			out = append(out, it.Name)
		}
	}
	return out, nil
}

func parseGitLabLatestRelease(raw string) (string, error) {
	var items []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return "", fmt.Errorf("gitlab releases json: %w", err)
	}
	if len(items) == 0 || items[0].TagName == "" {
		return "", fmt.Errorf("%w", foundation.ErrEmptyReleaseTag)
	}
	return items[0].TagName, nil
}

func filterRemminaTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if isRemminaReleaseTag(t) {
			out = append(out, t)
		}
	}
	return out
}

func isRemminaReleaseTag(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "v") {
		return false
	}
	v := foundation.ParseVersion(s)
	return !v.IsTrunk() && !v.IsLatest() && len(v.Parts) >= 2
}
