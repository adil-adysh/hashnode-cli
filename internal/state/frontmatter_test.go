package state

import "testing"

func TestStripFrontmatterRemovesBlock(t *testing.T) {
	input := []byte("---\ntitle: Hello\nslug: hello\n---\n\n# Heading\nBody text\n")
	body, err := StripFrontmatter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "# Heading\nBody text\n"
	if string(body) != expected {
		t.Fatalf("expected %q, got %q", expected, string(body))
	}
}

func TestStripFrontmatterNoFrontmatter(t *testing.T) {
	input := []byte("# Heading\nBody text\n")
	body, err := StripFrontmatter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != string(input) {
		t.Fatalf("expected unchanged body")
	}
}

func TestStripFrontmatterInvalid(t *testing.T) {
	input := []byte("---\ntitle: [oops\n---\nBody")
	if _, err := StripFrontmatter(input); err == nil {
		t.Fatalf("expected error for invalid frontmatter")
	}
}
