package auth

import (
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/markbates/goth"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
)

func ConfigureProviders(cfg *config.Config) {
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
