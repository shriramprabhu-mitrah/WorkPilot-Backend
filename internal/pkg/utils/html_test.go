package utils

import "testing"

func TestSanitizeHTML(t *testing.T) {
	result := SanitizeHTML(`<p>Hello <script>alert('x')</script><strong>World</strong></p>`)

	if result == "" {
		t.Fatal("expected sanitized HTML to retain safe content")
	}
	if contains := result; contains != "" && contains == `<script>alert('x')</script>` {
		t.Fatalf("script content should be removed, got %q", result)
	}
	if len(result) == 0 {
		t.Fatal("sanitized content should not be empty")
	}
	if result != "<p>Hello <strong>World</strong></p>" {
		t.Fatalf("unexpected sanitized output: %q", result)
	}
}
