package handlers

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/nerney/pt-dashboard/internal/config"
)

type Handler struct {
	store     *config.Store
	templates map[string]*template.Template
	fs        embed.FS
}

func NewRouter(store *config.Store, fs embed.FS) http.Handler {
	h := &Handler{store: store, fs: fs}
	h.templates = h.parseTemplates()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(noCache)

	r.Handle("/static/*", http.FileServer(http.FS(fs)))

	r.Get("/", h.dashboard)
	r.Get("/wizard/1", h.wizardStep1)
	r.Post("/wizard/1", h.wizardStep1Post)
	r.Get("/wizard/2", h.wizardStep2)
	r.Post("/wizard/2", h.wizardStep2Post)
	r.Get("/wizard/3", h.wizardStep3)
	r.Post("/wizard/3", h.wizardStep3Post)
	r.Post("/sync", h.sync)
	r.Get("/config", h.configPage)
	r.Post("/config/prowlarr", h.configProwlarrPost)
	r.Post("/config/tracker/{idx}/update", h.configTrackerUpdate)
	r.Post("/config/tracker/{idx}/prowlarr/add", h.configTrackerProwlarrAdd)
	r.Post("/config/tracker/{idx}/prowlarr/toggle", h.configTrackerProwlarrToggle)
	r.Post("/config/tracker/{idx}/prowlarr/remove", h.configTrackerProwlarrRemove)
	r.Post("/config/tracker/{idx}/delete", h.configTrackerDelete)

	return r
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) parseTemplates() map[string]*template.Template {
	funcs := templateFuncs()
	card := "templates/partials/tracker_card.html"

	parse := func(name string, files ...string) *template.Template {
		return template.Must(
			template.New(name).Funcs(funcs).ParseFS(h.fs, files...),
		)
	}

	return map[string]*template.Template{
		"wizard1":      parse("layout", "templates/layout.html", "templates/wizard1.html"),
		"wizard2":      parse("layout", "templates/layout.html", "templates/wizard2.html"),
		"wizard3":      parse("layout", "templates/layout.html", "templates/wizard3.html"),
		"dashboard":    parse("layout", "templates/layout.html", "templates/dashboard.html", card),
		"config":       parse("layout", "templates/layout.html", "templates/config.html"),
		"tracker_cards": parse("tracker_cards",
			"templates/partials/tracker_cards.html",
			card,
		),
	}
}

func (h *Handler) render(w http.ResponseWriter, page string, data interface{}) {
	t, ok := h.templates[page]
	if !ok {
		http.Error(w, "template not found: "+page, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func (h *Handler) renderPartial(w http.ResponseWriter, name string, data interface{}) {
	t, ok := h.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}
