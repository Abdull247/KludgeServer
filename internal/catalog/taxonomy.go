package catalog

import "strings"

const (
	GalleryBanners     = "banners"
	GalleryScreenshots = "screenshots"
)

type ItemTypeDef struct {
	Slug        string `json:"slug"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	CategoryID  string `json:"categoryId"`
	SectionID   string `json:"sectionId"`
	GalleryKind string `json:"galleryKind"`
}

var itemTypes = []ItemTypeDef{
	{Slug: "modded_apk", Label: "Mod APK", Kind: "build", CategoryID: "cat_modded_apk", SectionID: "sec_builds", GalleryKind: GalleryScreenshots},
	{Slug: "unofficial_api", Label: "Unofficial API", Kind: "api", CategoryID: "cat_agentic_api", SectionID: "sec_apis", GalleryKind: GalleryBanners},
	{Slug: "ai_jailbreak", Label: "AI Jailbreak", Kind: "build", CategoryID: "cat_ai_jailbreak", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "frida", Label: "Frida", Kind: "build", CategoryID: "cat_frida", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "custom_rom", Label: "Custom ROM", Kind: "build", CategoryID: "cat_custom_rom", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "ui_kit", Label: "UI Kit", Kind: "build", CategoryID: "cat_ui_kit", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "root_pack", Label: "Root Pack", Kind: "build", CategoryID: "cat_root", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "patch", Label: "Patch", Kind: "build", CategoryID: "cat_patch", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "prompt_pack", Label: "Prompt Pack", Kind: "build", CategoryID: "cat_prompt", SectionID: "sec_builds", GalleryKind: GalleryBanners},
	{Slug: "graphql", Label: "GraphQL", Kind: "api", CategoryID: "cat_graphql", SectionID: "sec_apis", GalleryKind: GalleryBanners},
	{Slug: "proxy", Label: "Proxy", Kind: "api", CategoryID: "cat_proxy", SectionID: "sec_apis", GalleryKind: GalleryBanners},
	{Slug: "sdk", Label: "SDK", Kind: "api", CategoryID: "cat_sdk", SectionID: "sec_apis", GalleryKind: GalleryBanners},
}

func GalleryKindFor(slug string) string {
	if def, ok := LookupType(slug); ok && def.GalleryKind != "" {
		return def.GalleryKind
	}
	return GalleryBanners
}

func IsAPK(slug string) bool {
	return GalleryKindFor(slug) == GalleryScreenshots
}

func TypeDefs() []ItemTypeDef {
	out := make([]ItemTypeDef, len(itemTypes))
	copy(out, itemTypes)
	return out
}

func LookupType(v string) (ItemTypeDef, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, t := range itemTypes {
		if t.Slug == v || strings.EqualFold(t.Label, v) {
			return t, true
		}
	}
	return ItemTypeDef{}, false
}
