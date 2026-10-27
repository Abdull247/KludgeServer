package catalog

import "strings"

type ItemTypeDef struct {
	Slug       string `json:"slug"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	CategoryID string `json:"categoryId"`
	SectionID  string `json:"sectionId"`
}

var itemTypes = []ItemTypeDef{
	{Slug: "modded_apk", Label: "Mod APK", Kind: "build", CategoryID: "cat_modded_apk", SectionID: "sec_builds"},
	{Slug: "unofficial_api", Label: "Unofficial API", Kind: "api", CategoryID: "cat_agentic_api", SectionID: "sec_apis"},
	{Slug: "ai_jailbreak", Label: "AI Jailbreak", Kind: "build", CategoryID: "cat_ai_jailbreak", SectionID: "sec_builds"},
	{Slug: "frida", Label: "Frida", Kind: "build", CategoryID: "cat_frida", SectionID: "sec_builds"},
	{Slug: "custom_rom", Label: "Custom ROM", Kind: "build", CategoryID: "cat_custom_rom", SectionID: "sec_builds"},
	{Slug: "ui_kit", Label: "UI Kit", Kind: "build", CategoryID: "cat_ui_kit", SectionID: "sec_builds"},
	{Slug: "root_pack", Label: "Root Pack", Kind: "build", CategoryID: "cat_root", SectionID: "sec_builds"},
	{Slug: "patch", Label: "Patch", Kind: "build", CategoryID: "cat_patch", SectionID: "sec_builds"},
	{Slug: "prompt_pack", Label: "Prompt Pack", Kind: "build", CategoryID: "cat_prompt", SectionID: "sec_builds"},
	{Slug: "graphql", Label: "GraphQL", Kind: "api", CategoryID: "cat_graphql", SectionID: "sec_apis"},
	{Slug: "proxy", Label: "Proxy", Kind: "api", CategoryID: "cat_proxy", SectionID: "sec_apis"},
	{Slug: "sdk", Label: "SDK", Kind: "api", CategoryID: "cat_sdk", SectionID: "sec_apis"},
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
