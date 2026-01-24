package store

import "testing"

func TestParseIdeaContentNormalizesTags(t *testing.T) {
	input := "Idea #idea\n" +
		"tags: #alpha, @beta, +gamma\n" +
		"\n" +
		"Body with #delta and @echo and +skip\n"
	_, tags, _ := parseIdeaContent(input)
	want := []string{"alpha", "beta", "delta", "echo", "gamma", "idea"}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %d: %#v", len(want), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Fatalf("expected tag %q at index %d, got %q", want[i], i, tag)
		}
	}
}

func TestExtractIdeaInlineTagsSkipsCodeFences(t *testing.T) {
	input := "Keep #one\n```go\n#two @three\n```\nAfter @four\n"
	tags := extractIdeaInlineTags(input)
	want := []string{"one", "four"}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %d: %#v", len(want), len(tags), tags)
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Fatalf("expected tag %q at index %d, got %q", want[i], i, tag)
		}
	}
}
