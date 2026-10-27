package catalog

import (
	"encoding/json"
	"os"
	"sync"
)

type Tag struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

type tagFile struct {
	Tags []Tag `json:"tags"`
}

var (
	tagsOnce sync.Once
	tagsMem  []Tag
)

func loadTags() []Tag {
	tagsOnce.Do(func() {
		for _, p := range []string{
			"data/tags.json",
			"internal/catalog/data/tags.json",
		} {
			b, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if parsed := parseTags(b); len(parsed) > 0 {
				tagsMem = parsed
				return
			}
		}
		tagsMem = parseTags(embeddedTags)
		if len(tagsMem) == 0 {
			tagsMem = []Tag{{Slug: "android", Label: "Android"}, {Slug: "apk", Label: "APK"}}
		}
	})
	out := make([]Tag, len(tagsMem))
	copy(out, tagsMem)
	return out
}

func parseTags(b []byte) []Tag {
	var f tagFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil
	}
	out := make([]Tag, 0, len(f.Tags))
	for _, t := range f.Tags {
		if t.Slug == "" || t.Label == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}
