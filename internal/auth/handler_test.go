package auth

import (
	"net/url"
	"testing"

	"github.com/Abdull247/KludgeServer/internal/config"
)

func TestSafeNext(t *testing.T) {
	h := &Handler{cfg: &config.Config{FrontendURL: "https://kludgekit.web.app"}}
	cases := map[string]bool{
		"":                                    false,
		"//evil.com":                          false,
		"/home":                               true,
		"/onboarding?welcome=1":               true,
		"https://kludgekit.web.app/home":      true,
		"https://kludgekit.firebaseapp.com/x": true,
		"http://evil.com/home":                false,
		"https://evil.com/home":               false,
		"javascript:alert(1)":                 false,
	}
	for in, want := range cases {
		if got := h.safeNext(in); got != want {
			t.Fatalf("safeNext(%q)=%v want %v", in, got, want)
		}
	}
}

func TestAppendQuery(t *testing.T) {
	got := appendQuery("https://kludgekit.web.app/home", url.Values{
		"welcome": {"1"},
		"session": {"tok"},
	})
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("welcome") != "1" || u.Query().Get("session") != "tok" {
		t.Fatalf("got %q", got)
	}
}
