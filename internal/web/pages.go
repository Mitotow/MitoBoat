package web

import (
	"html/template"
	"log/slog"
	"net/http"
)

type link struct {
	Href  string
	Label string
}

type pageData struct {
	Title string
	Body  string
	Links []link
}

// pageTemplate renders every response. html/template escapes the values, so a
// Twitch display name cannot inject markup into the page.
var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} &middot; MitoBoat</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 34rem; margin: 4rem auto;
         padding: 0 1.25rem; line-height: 1.6; color: #1c1b22; background: #faf9fb; }
  h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
  a.button { display: inline-block; margin-top: 1.5rem; padding: 0.6rem 1.1rem;
             background: #9146ff; color: #fff; border-radius: 6px; text-decoration: none; }
  a.button:hover { background: #7d2ff5; }
  @media (prefers-color-scheme: dark) {
    body { color: #eceaf0; background: #18171b; }
  }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
<p>{{.Body}}</p>
{{range .Links}}<a class="button" href="{{.Href}}">{{.Label}}</a>{{end}}
</body>
</html>
`))

// render writes a page. The status is set before the body, and a template
// failure is only logged: the header has already gone out by then.
func render(w http.ResponseWriter, status int, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := pageTemplate.Execute(w, data); err != nil {
		slog.Error("Could not render a page", "scope", "WEB", "error", err)
	}
}
