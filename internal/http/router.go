package httpx

import (
	"net/http"

	"github.com/Abdull247/KludgeServer/internal/admin"
	"github.com/Abdull247/KludgeServer/internal/auth"
	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/profile"
	"github.com/Abdull247/KludgeServer/internal/upload"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg *config.Config, authHandler *auth.Handler, profileHandler *profile.Handler, catalogHandler *catalog.Handler, adminHandler *admin.Handler, uploadHandler *upload.Handler) http.Handler {
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

		if catalogHandler != nil {
			r.Route("/v1", func(r chi.Router) {
				r.Get("/home", catalogHandler.Home)
				r.Get("/sections/{id}", catalogHandler.Section)
				r.Get("/categories/{id}", catalogHandler.Category)
				r.Get("/trending", catalogHandler.Trending)
				r.Get("/dropped", catalogHandler.Dropped)
				r.Get("/collections", catalogHandler.Collections)
				r.Get("/search", catalogHandler.Search)
				r.Get("/types", catalogHandler.Types)
				r.Get("/tags", catalogHandler.Tags)
				r.Post("/items", catalogHandler.CreateItem)
				r.Post("/items/record", catalogHandler.RecordItem)
				r.Get("/items/{id}", catalogHandler.Item)
				r.Get("/items/{id}/recommends", catalogHandler.Recommends)
				r.Post("/items/{id}/save", catalogHandler.Save)
				r.Post("/items/{id}/like", catalogHandler.Like)

				if uploadHandler != nil {
					r.Get("/uploads/limits", uploadHandler.Limits)
					r.Post("/uploads", uploadHandler.Put)
				}

				if adminHandler != nil {
					r.Route("/admin", func(r chi.Router) {
						r.Get("/home", adminHandler.Home)
						r.Get("/users", adminHandler.Users)
						r.Get("/projects", adminHandler.Projects)
					})
				}
			})
		}
	})

	return r
}
