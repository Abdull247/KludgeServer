package user

import (
	"context"
	"os"
	"testing"

	"github.com/Abdull247/KludgeServer/internal/database"
)

func TestUpsertFromOAuth(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://kludge:kludge_dev@127.0.0.1:5432/kludge?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, url)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	store := NewStore(pool)
	email := "test-oauth@kludge.dev"

	_, _ = pool.Exec(ctx, `DELETE FROM oauth_accounts WHERE provider_user_id IN ('gh-test-1', 'google-test-1')`)
	_, _ = pool.Exec(ctx, `DELETE FROM profiles WHERE email = $1`, email)
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE email = $1`, email)

	u, err := store.UpsertFromOAuth(ctx, UpsertOAuthInput{
		Provider:       "github",
		ProviderUserID: "gh-test-1",
		Email:          email,
		Name:           "Test Dev",
		AvatarURL:      "https://example.com/a.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != email {
		t.Fatalf("email: %s", u.Email)
	}

	again, err := store.UpsertFromOAuth(ctx, UpsertOAuthInput{
		Provider:       "github",
		ProviderUserID: "gh-test-1",
		Email:          email,
		Name:           "Test Dev Updated",
		AvatarURL:      "https://example.com/b.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != u.ID {
		t.Fatal("expected same user id on second github login")
	}
	if again.Name != "Test Dev Updated" {
		t.Fatalf("name not updated: %s", again.Name)
	}

	linked, err := store.UpsertFromOAuth(ctx, UpsertOAuthInput{
		Provider:       "google",
		ProviderUserID: "google-test-1",
		Email:          email,
		Name:           "Test Dev",
		AvatarURL:      "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if linked.ID != u.ID {
		t.Fatal("expected google account to link to the same user")
	}

	loaded, err := store.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.Email != email {
		t.Fatal("GetByID failed")
	}
}
