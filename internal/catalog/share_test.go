package catalog

import (
	"strings"
	"testing"
)

func TestClampShareText(t *testing.T) {
	if got := clampShareText("  hello   world  ", 100); got != "hello world" {
		t.Fatalf("normalize: %q", got)
	}
	long := strings.Repeat("word ", 200)
	got := clampShareText(long, 80)
	base := strings.TrimSuffix(got, "…")
	if len(base) > 80 || len(got) == len(base) {
		t.Fatalf("clamp: len %d tail %q", len(got), got)
	}
	if !strings.HasPrefix(got, "word") {
		t.Fatalf("clamp head: %q", got)
	}
}
