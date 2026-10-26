package httpx

import (
	"net/http"

	"github.com/Abdull247/KludgeServer/internal/auth"
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/profile"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg *config.Config, authHandler *auth.Handler, profileHandler *profile.Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(Logger)
	r.Use(middleware.Recoverer)
	r.Use(CORS(cfg))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Get("/me", authHandler.Me)
			r.Post("/logout", authHandler.Logout)
			r.Get("/{provider}", authHandler.Begin)
			r.Get("/{provider}/callback", authHandler.Callback)
		})
		r.Route("/profile", func(r chi.Router) {
			r.Get("/me", profileHandler.Me)
			r.Post("/onboarding", profileHandler.CompleteOnboarding)
		})
	})

	return r
}
