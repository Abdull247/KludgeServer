package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/auth"
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/google/uuid"
)

// mint prints a bearer token for the given user id, signed with the exact
// SESSION_SECRET / SESSION_COOKIE the server loads (env beats .env).
// The printed value also works as the session cookie value.
func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal(fmt.Sprintf("config: %v", err))
	}
	if len(os.Args) < 2 {
		fatal("usage: go run ./cmd/mint <userId>")
	}
	userID, err := uuid.Parse(strings.TrimSpace(os.Args[1]))
	if err != nil {
		fatal("userId must be a UUID")
	}
	sessions := auth.NewSessionStore(cfg)
	token, err := sessions.Token(userID)
	if err != nil {
		fatal(fmt.Sprintf("mint: %v", err))
	}
	fmt.Println(token)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
