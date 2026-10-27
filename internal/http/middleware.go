package httpx

import (
	"log"
	"net/http"
	"time"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/go-chi/cors"
)

func CORS(cfg *config.Config) func(http.Handler) http.Handler {
	opts := cors.Options{
		AllowedOrigins:   []string{cfg.FrontendURL},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}
	if cfg.IsDev() || cfg.IsTest() {
		opts.AllowedOrigins = nil
		opts.AllowOriginFunc = func(r *http.Request, origin string) bool {
			return true
		}
	}
	return cors.Handler(opts)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &wrapWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, ww.status, time.Since(start).Truncate(time.Millisecond))
	})
}

type wrapWriter struct {
	http.ResponseWriter
	status int
}

func (w *wrapWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
