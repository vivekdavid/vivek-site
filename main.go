package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/yuin/goldmark"
	"wiki/db"
)

var (
	sessionManager *scs.SessionManager
	templates      = template.Must(template.New("").Funcs(template.FuncMap{"titleToSlug": titleToSlug}).ParseFiles("templates/home.html", "edit.html", "view.html", "templates/login.html", "templates/profile.html", "templates/titles.html", "templates/create-lecture.html", "templates/create-theme.html"))
	// Allows case-insensitive routing for edit, save, and view
	// validPath = regexp.MustCompile("(?i)^/(edit|save|view)/([a-zA-Z0-9-]+)$")
	// )
	// character a title is allowed to contain can show up here too.
	validPath           = regexp.MustCompile("(?i)^/(edit|save|view)/([^/]+)$")
	titleForbiddenChars = regexp.MustCompile(`[-/]`)
	mdRenderer          = goldmark.New()
)

type PageData struct {
	Error string
}

type ViewData struct {
	Title string
	Body  template.HTML
}

func slugToTitle(slug string) string {
	return strings.ReplaceAll(slug, "-", " ")
}

func titleToSlug(title string) string {
	return strings.ReplaceAll(strings.TrimSpace(title), " ", "-")
}

func renderTemplate(w http.ResponseWriter, tmpl string, p interface{}) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Handler wrapper that handles route regex checking and authentication checks
func makeHandler(fn func(http.ResponseWriter, *http.Request, string), requireAuth bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		if requireAuth {
			username := sessionManager.GetString(r.Context(), "username")
			if username == "" {
				http.Redirect(w, r, "/wiki/login?redirect=/wiki"+r.URL.Path, http.StatusSeeOther)
				return
			}
		}
		fn(w, r, m[2])
	}
}

// regular homepage handlers
func homeHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "home", nil)
}

// wikihandlers start here
func viewHandler(w http.ResponseWriter, r *http.Request, slug string) {
	title := slugToTitle(slug)
	p, err := database.GetPage(title)
	if err != nil {
		http.Redirect(w, r, "/wiki/edit/"+slug, http.StatusFound)
		return
	}
	var rendered bytes.Buffer
	if err := mdRenderer.Convert(p.Body, &rendered); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	renderTemplate(w, "view", ViewData{
		Title: p.Title,
		Body:  template.HTML(rendered.String()),
	})
}

func editHandler(w http.ResponseWriter, r *http.Request, slug string) {
	title := slugToTitle(slug)
	p, err := database.GetPage(title)
	if err != nil {
		// Initialize struct with default values if page doesn't exist yet
		p = &database.Page{Title: title}
	}

	renderTemplate(w, "edit", p)
}

func createLectureHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, "create-lecture", nil)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	dateStr := r.FormValue("date")
	body := r.FormValue("body")

	if title == "" || dateStr == "" {
		renderTemplate(w, "create-lecture", PageData{Error: "Title and date are required"})
		return
	}

	if titleForbiddenChars.MatchString(title) {
		renderTemplate(w, "create-lecture", PageData{Error: "Titles cannot contain hyphens or slashes"})
		return
	}
	entryDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		renderTemplate(w, "create-lecture", PageData{Error: "Invalid date"})
		return
	}

	// NEW: reject dates outside the allowed range
	minDate := time.Date(1979, time.September, 1, 0, 0, 0, 0, time.UTC)
	maxDate := time.Date(1984, time.October, 31, 0, 0, 0, 0, time.UTC)
	if entryDate.Before(minDate) || entryDate.After(maxDate) {
		renderTemplate(w, "create-lecture", PageData{Error: "Date must be between September 1979 and October 1984"})
		return
	}

	// UNCHANGED: your existing save logic
	if err := database.CreatePage(title, []byte(body), "lecture", &entryDate); err != nil {
		renderTemplate(w, "create-lecture", PageData{Error: "A page with that title already exists"})
		return
	}
	http.Redirect(w, r, "/wiki/view/"+titleToSlug(title), http.StatusFound)
}

// createThemeHandler handles the "theme" entry type: title + body, no date.
func createThemeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, "create-theme", nil)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	body := r.FormValue("body")

	if title == "" {
		renderTemplate(w, "create-theme", PageData{Error: "Title is required"})
		return
	}

	if titleForbiddenChars.MatchString(title) {
		renderTemplate(w, "create-theme", PageData{Error: "Titles cannot contain hyphens or slashes"})
		return
	}

	if err := database.CreatePage(title, []byte(body), "theme", nil); err != nil {
		renderTemplate(w, "create-theme", PageData{Error: "A page with that title already exists"})
		return
	}

	http.Redirect(w, r, "/wiki/view/"+titleToSlug(title), http.StatusFound)
}

func saveHandler(w http.ResponseWriter, r *http.Request, slug string) {
	body := r.FormValue("body")
	title := slugToTitle(slug)

	err := database.SavePage(title, []byte(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/wiki/view/"+titleToSlug(title), http.StatusFound)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplate(w, "login", nil)
		return
	}

	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		password := r.FormValue("password")

		dbPassword, err := database.GetUserPassword(username)
		if err != nil || dbPassword != password {
			renderTemplate(w, "login", PageData{Error: "Invalid username or password"})
			return
		}

		sessionManager.RenewToken(r.Context())
		sessionManager.Put(r.Context(), "username", username)

		target := r.URL.Query().Get("redirect")
		if target == "" {
			target = "/wiki/contents" // Fallback default
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	username := sessionManager.GetString(r.Context(), "username")
	if username == "" {
		http.Redirect(w, r, "/wiki/login", http.StatusSeeOther)
		return
	}

	type ProfileData struct{ Username string }
	renderTemplate(w, "profile", ProfileData{Username: username})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	err := sessionManager.Destroy(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/wiki/login", http.StatusSeeOther)
}

func requireAuth(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := sessionManager.GetString(r.Context(), "username")
		if username == "" {
			http.Redirect(w, r, "/wiki/login?redirect=/wiki"+r.URL.Path, http.StatusSeeOther)
			return
		}
		fn(w, r)
	}
}

type TitlesData struct {
	Lectures []database.Page
	Themes   []database.Page
}

func titlesHandler(w http.ResponseWriter, r *http.Request) {
	pages, err := database.GetTitles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data TitlesData
	for _, p := range pages {
		if p.EntryType == "lecture" {
			data.Lectures = append(data.Lectures, p)
		} else {
			data.Themes = append(data.Themes, p)
		}
	}
	// Most recent lecture first; themes stay alphabetical (already sorted by the query).
	sort.Slice(data.Lectures, func(i, j int) bool {
		return data.Lectures[i].EntryDate.Time.After(data.Lectures[j].EntryDate.Time)
	})

	renderTemplate(w, "titles", data)
}

func main() {
	database.InitDB()
	defer database.DB.Close()

	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode
	sessionManager.Cookie.Secure = false

	// ----- this is for regular not wiki prefix
	mux := http.NewServeMux()

	mux.HandleFunc("/", homeHandler)
	wikiMux := http.NewServeMux()
	// wiki routes
	// Auth routes
	wikiMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	wikiMux.HandleFunc("/login", loginHandler)
	wikiMux.HandleFunc("/logout", logoutHandler)
	wikiMux.HandleFunc("/profile", profileHandler)
	wikiMux.HandleFunc("/", titlesHandler)
	wikiMux.HandleFunc("/view/", makeHandler(viewHandler, false))
	wikiMux.HandleFunc("/edit/", makeHandler(editHandler, true))
	wikiMux.HandleFunc("/save/", makeHandler(saveHandler, true))
	wikiMux.HandleFunc("/create/lecture", requireAuth(createLectureHandler))
	wikiMux.HandleFunc("/create/theme", requireAuth(createThemeHandler))

	mux.Handle("/wiki/", http.StripPrefix("/wiki", wikiMux))

	fmt.Println("Server starting at :8000...")
	log.Fatal(http.ListenAndServe(":8000", sessionManager.LoadAndSave(mux)))
}
