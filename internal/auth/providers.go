package auth

import (
	"log"
	"net/url"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
)

func ConfigureProviders(cfg *config.Config) {
	githubOK := cfg.GitHubClientID != "" && cfg.GitHubClientSecret != ""
	googleOK := cfg.GoogleClientID != "" && cfg.GoogleClientSecret != ""
	log.Printf("auth: github=%s google=%s (needs GITHUB_/GOOGLE_CLIENT_ID+SECRET in the server process env at startup)",
		ready(githubOK), ready(googleOK))
	if !googleOK {
		log.Printf("auth: google unconfigured — set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET in KludgeServer/.env, then RESTART the server; redirect URI in Google Console must be %s", cfg.CallbackURL("google"))
	}
	if cfg.FrontendURL != "" {
		if u, err := url.Parse(cfg.FrontendURL); err == nil && (strings.HasSuffix(u.Host, ":5173") || strings.HasSuffix(u.Host, ":5174")) {
			log.Printf("auth: FRONTEND_URL=%s points at the old Vite client; the web client is Next.js, expected http://localhost:3000", cfg.FrontendURL)
		}
	}

	goth.UseProviders(
		github.New(
			cfg.GitHubClientID,
			cfg.GitHubClientSecret,
			cfg.CallbackURL("github"),
			"user:email",
		),
		google.New(
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.CallbackURL("google"),
			"email",
			"profile",
		),
	)
}

func ready(ok bool) string {
	if ok {
		return "configured"
	}
	return "not-configured"
}
