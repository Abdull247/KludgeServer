package auth

import (
	"net/http"
	"net/url"
	"strings"
)

const (
	clientWeb = "web"
	clientApp = "app"
)

func detectClient(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Kludge-Client")); v != "" {
		return normalizeClient(v)
	}
	if v := strings.TrimSpace(r.Header.Get("X-Client")); v != "" {
		return normalizeClient(v)
	}
	if v := strings.TrimSpace(r.URL.Query().Get("client")); v != "" {
		return normalizeClient(v)
	}
	return clientWeb
}

func normalizeClient(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "app", "android", "ios", "mobile", "native":
		return clientApp
	default:
		return clientWeb
	}
}

func isAppClient(value string) bool {
	return normalizeClient(value) == clientApp
}

func appDeepLink(appLink, ticket string) string {
	base := strings.TrimSpace(appLink)
	if base == "" {
		base = "kludge://auth/complete"
	}
	if ticket == "" {
		return base
	}
	u, err := url.Parse(base)
	if err != nil {
		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		return base + sep + "ticket=" + url.QueryEscape(ticket)
	}
	q := u.Query()
	q.Set("ticket", ticket)
	u.RawQuery = q.Encode()
	return u.String()
}

func appIntentLink(packageName, ticket string) string {
	pkg := strings.TrimSpace(packageName)
	if pkg == "" {
		pkg = "com.kludge.app"
	}
	path := "auth/complete"
	if ticket != "" {
		path += "?ticket=" + url.QueryEscape(ticket)
	}
	return "intent://" + path + "#Intent;scheme=kludge;package=" + pkg + ";end"
}
