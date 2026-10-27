package upload

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/Abdull247/KludgeServer/internal/config"
	"github.com/Abdull247/KludgeServer/internal/respond"
	"github.com/Abdull247/KludgeServer/internal/storage"
	"github.com/google/uuid"
)

type SessionReader interface {
	UserID(r *http.Request) (uuid.UUID, bool)
}

type Handler struct {
	sessions SessionReader
	store    *storage.Store
	cfg      *config.Config
}

func NewHandler(sessions SessionReader, store *storage.Store, cfg *config.Config) *Handler {
	return &Handler{sessions: sessions, store: store, cfg: cfg}
}

type Limits struct {
	MaxBytes       int64    `json:"maxBytes"`
	MaxDisplay     string   `json:"maxDisplay"`
	ImageTypes     []string `json:"imageTypes"`
	SourceTypes    []string `json:"sourceTypes"`
	ZipForMultiple bool     `json:"zipForMultiple"`
}

func (h *Handler) Limits(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, h.limits())
}

func (h *Handler) limits() Limits {
	max := h.cfg.MaxUploadBytes
	return Limits{
		MaxBytes:       max,
		MaxDisplay:     formatBytes(max),
		ImageTypes:     []string{"image/png", "image/jpeg", "image/webp", "image/gif"},
		SourceTypes:    []string{"application/zip", "application/x-zip-compressed", "application/octet-stream"},
		ZipForMultiple: true,
	}
}

func (h *Handler) requireUser(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := h.sessions.UserID(r); !ok {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return false
	}
	return true
}

func (h *Handler) Put(w http.ResponseWriter, r *http.Request) {
	if !h.requireUser(w, r) {
		return
	}
	if h.store == nil || !h.store.Configured() {
		respond.Error(w, http.StatusServiceUnavailable, "uploads are not configured")
		return
	}

	kind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	switch kind {
	case "artwork", "banner", "source":
	default:
		kind = "source"
	}

	max := h.cfg.MaxUploadBytes
	r.Body = http.MaxBytesReader(w, r.Body, max+512)
	if err := r.ParseMultipartForm(max); err != nil {
		respond.Error(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds the %s limit", formatBytes(max)))
		return
	}

	file, hdr, err := r.FormFile("file")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	if hdr.Size > max {
		respond.Error(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds the %s limit", formatBytes(max)))
		return
	}

	body, err := io.ReadAll(io.LimitReader(file, max+1))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "could not read file")
		return
	}
	if int64(len(body)) > max {
		respond.Error(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("file exceeds the %s limit", formatBytes(max)))
		return
	}

	contentType := hdr.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	if err := validateKind(kind, contentType, hdr.Filename); err != nil {
		respond.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	key := storage.NewKey(kind, storage.SafeFilename(hdr.Filename))
	obj, err := h.store.Put(r.Context(), key, body, contentType)
	if err != nil {
		respond.Error(w, http.StatusBadGateway, "could not store file")
		return
	}
	obj.Kind = kind
	respond.JSON(w, http.StatusCreated, obj)
}

func validateKind(kind, contentType, filename string) error {
	ext := strings.ToLower(path.Ext(filename))
	switch kind {
	case "artwork", "banner":
		if strings.HasPrefix(contentType, "image/") || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".gif" {
			return nil
		}
		return fmt.Errorf("only image files are allowed")
	case "source":
		if filename == "" {
			return fmt.Errorf("source file is required")
		}
		return nil
	}
	return nil
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	kb := float64(n) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.0f KB", kb)
	}
	mb := kb / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.0f MB", mb)
	}
	return fmt.Sprintf("%.1f GB", mb/1024)
}
