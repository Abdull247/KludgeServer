package auth

import (
	"html"
	"net/http"
	"strings"
)

func handoffPage(deepLink, intentLink string) string {
	safeDeep := html.EscapeString(deepLink)
	return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Kludge</title>
</head>
<body style="margin:0;background:#0b0b0b;color:#f3f3f3;font-family:sans-serif;display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:100vh;gap:16px">
  <p>Opening Kludge...</p>
  <p><a href="` + safeDeep + `" style="color:#7c5cff">Tap here if the app does not open</a></p>
  <script>
    (function () {
      var app = ` + jsString(deepLink) + `;
      var intent = ` + jsString(intentLink) + `;
      try { location.replace(intent); } catch (e) {}
      setTimeout(function () {
        try { location.replace(app); } catch (e) {}
      }, 200);
    })();
  </script>
</body>
</html>`
}

func jsString(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`), "\n", `\n`) + `"`
}

func writeHandoffPage(w http.ResponseWriter, deepLink, intentLink string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(handoffPage(deepLink, intentLink)))
}
