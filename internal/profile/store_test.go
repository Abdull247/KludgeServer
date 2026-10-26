package profile

import "testing"

func TestFirstNameSlug(t *testing.T) {
	cases := map[string]string{
		"Amaka Okonkwo": "amaka",
		"  Ada-eze  ":   "ada",
		"":              "user",
		"@@@":           "user",
		"X":             "x",
	}
	for in, want := range cases {
		if got := firstNameSlug(in); got != want {
			t.Fatalf("firstNameSlug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestGenerateUsername(t *testing.T) {
	u := generateUsername("Ada Lovelace")
	if len(u) != len("ada")+6 {
		t.Fatalf("len=%d value=%s", len(u), u)
	}
	if u[:3] != "ada" {
		t.Fatalf("prefix: %s", u)
	}
}
