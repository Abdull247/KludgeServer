package share

import (
	"html/template"
	"net/http"
	"net/url"

	"github.com/Abdull247/KludgeServer/internal/catalog"
	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/go-chi/chi/v5"
)

// Source resolves the share-card details for one item.
type Source interface {
	// Share returns a catalog.SharePreview when the item is public.
	Share(idOrSlug string) (catalog.SharePreview, bool)
}

var _ Source = (*catalog.Store)(nil)

type pageData struct {
	Title       string
	Description string
	Image       string
	Target      string
	Canonical   string
	HasImage    bool
}

// Handler serves crawler-friendly HTML for item share links on /i/{id}.
// Social apps (WhatsApp, Telegram, X) read the Open Graph tags from this
// page, while people are handed straight to the app via a meta refresh.
type Handler struct {
	cfg    *config.Config
	source Source
	page   *template.Template
}

func NewHandler(cfg *config.Config, source Source) *Handler {
	tmpl := template.Must(template.New("share").Parse(pageHTML))
	return &Handler{cfg: cfg, source: source, page: tmpl}
}

func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
		return
	}

	preview, ok := h.source.Share(id)
	if !ok {
		http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
		return
	}

	target := h.cfg.FrontendURL + "/item?id=" + url.QueryEscape(id)
	data := pageData{
		Title:       preview.Title,
		Description: preview.Description,
		Image:       preview.Image,
		Target:      target,
		Canonical:   target,
		HasImage:    preview.Image != "",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	if err := h.page.Execute(w, data); err != nil {
		http.Redirect(w, r, target, http.StatusFound)
	}
}

const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} — Kludge</title>
<meta name="description" content="{{.Description}}">
<link rel="canonical" href="{{.Canonical}}">
<meta property="og:type" content="website">
<meta property="og:site_name" content="Kludge">
<meta property="og:title" content="{{.Title}} — Kludge">
<meta property="og:description" content="{{.Description}}">
<meta property="og:url" content="{{.Canonical}}">
{{if .HasImage}}<meta property="og:image" content="{{.Image}}">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Title}} — Kludge">
<meta name="twitter:description" content="{{.Description}}">
<meta name="twitter:image" content="{{.Image}}">
{{else}}<meta name="twitter:card" content="summary">
{{end}}<meta http-equiv="refresh" content="0; url={{.Target}}">
<script>location.replace({{.Target}});</script>
</head>
<body style="margin:0;min-height:100vh;display:grid;place-items:center;background:oklch(0.11 0.008 250);color:oklch(0.94 0.01 250);font-family:ui-sans-serif,system-ui,sans-serif">
<p style="opacity:.75">Opening Kludge… <a href="{{.Target}}" style="color:oklch(0.72 0.16 240)">Continue</a></p>
</body>
</html>
`
