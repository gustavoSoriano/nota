package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/soriano/nota/internal/domain"
	"github.com/soriano/nota/internal/usecase"
)

type Server struct {
	docRepo  domain.DocumentRepository
	linkRepo domain.DocumentLinkRepository
	static   embed.FS
}

func New(docRepo domain.DocumentRepository, linkRepo domain.DocumentLinkRepository, static embed.FS) *Server {
	return &Server{
		docRepo:  docRepo,
		linkRepo: linkRepo,
		static:   static,
	}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	staticDirs := []string{"internal/server/static", "static"}
	var staticHandler http.Handler
	for _, d := range staticDirs {
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			staticHandler = http.FileServer(http.Dir(d))
			break
		}
	}
	if staticHandler == nil {
		sub, err := fs.Sub(s.static, "static")
		if err != nil {
			return fmt.Errorf("static files: %w", err)
		}
		staticHandler = http.FileServer(http.FS(sub))
	}
	mux.Handle("/", staticHandler)

	mux.HandleFunc("GET /api/notes", s.handleListNotes)
	mux.HandleFunc("GET /api/notes/", s.handleGetNote)
	mux.HandleFunc("POST /api/notes", s.handleCreateNote)
	mux.HandleFunc("PUT /api/notes/", s.handleUpdateNote)
	mux.HandleFunc("DELETE /api/notes/", s.handleDeleteNote)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/tags", s.handleTags)
	mux.HandleFunc("POST /api/link", s.handleLink)

	handler := corsMiddleware(mux)

	fmt.Printf("Nota serving on http://localhost%s\n", addr)
	openBrowser("http://localhost" + addr)

	srv := &http.Server{Addr: addr, Handler: handler}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	return srv.ListenAndServe()
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	uc := usecase.NewListUseCase(s.docRepo)

	var tags []string
	if t := r.URL.Query().Get("tags"); t != "" {
		tags = strings.Split(t, ",")
	}

	docs, err := uc.Execute(r.Context(), usecase.ListInput{
		Tags:     tags,
		Notebook: r.URL.Query().Get("notebook"),
		Category: r.URL.Query().Get("category"),
		Sort:     r.URL.Query().Get("sort"),
		Limit:    200,
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	type noteItem struct {
		ID        string   `json:"id"`
		Title     string   `json:"title"`
		Preview   string   `json:"preview"`
		Tags      []string `json:"tags"`
		Notebook  string   `json:"notebook"`
		Category  string   `json:"category"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
		Accessed  int      `json:"accessed"`
	}

	items := make([]noteItem, len(docs))
	for i, d := range docs {
		preview := strings.ReplaceAll(d.Content, "\n", " ")
		preview = strings.TrimSpace(preview)
		if len(preview) > 120 {
			preview = preview[:117] + "..."
		}
		items[i] = noteItem{
			ID: d.ID, Title: d.Title, Preview: preview,
			Tags: d.Tags, Notebook: d.Notebook, Category: d.Category,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
			UpdatedAt: d.UpdatedAt.Format(time.RFC3339),
			Accessed:  d.Accessed,
		}
	}
	writeJSON(w, 200, items)
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if id == "" {
		writeError(w, 400, "missing id")
		return
	}

	uc := usecase.NewOpenUseCase(s.docRepo)
	doc, err := uc.Execute(r.Context(), id)
	if err != nil {
		writeError(w, 404, "not found")
		return
	}

	s.docRepo.IncrementAccessed(r.Context(), doc.ID)

	linked, _ := s.linkRepo.GetLinked(r.Context(), doc.ID)
	type linkedNote struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	type noteDetail struct {
		ID        string       `json:"id"`
		Title     string       `json:"title"`
		Content   string       `json:"content"`
		Tags      []string     `json:"tags"`
		Notebook  string       `json:"notebook"`
		Category  string       `json:"category"`
		CreatedAt string       `json:"created_at"`
		UpdatedAt string       `json:"updated_at"`
		Accessed  int          `json:"accessed"`
		Linked    []linkedNote `json:"linked"`
	}

	linkedItems := make([]linkedNote, len(linked))
	for i, l := range linked {
		linkedItems[i] = linkedNote{ID: l.ID, Title: l.Title}
	}

	writeJSON(w, 200, noteDetail{
		ID: doc.ID, Title: doc.Title, Content: doc.Content,
		Tags: doc.Tags, Notebook: doc.Notebook, Category: doc.Category,
		CreatedAt: doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt: doc.UpdatedAt.Format(time.RFC3339),
		Accessed:  doc.Accessed, Linked: linkedItems,
	})
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Notebook string   `json:"notebook"`
		Category string   `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}

	uc := usecase.NewSaveUseCase(s.docRepo)
	doc, err := uc.Execute(r.Context(), usecase.SaveInput{
		Content: body.Content, Tags: body.Tags,
		Notebook: body.Notebook, Category: body.Category,
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 201, map[string]string{"id": doc.ID, "title": doc.Title})
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if id == "" {
		writeError(w, 400, "missing id")
		return
	}

	var body struct {
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Notebook string   `json:"notebook"`
		Category string   `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}

	doc, err := s.docRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, 404, "not found")
		return
	}

	if body.Content != "" {
		doc.Content = body.Content
		doc.Title = doc.ExtractTitle()
	}
	if body.Tags != nil {
		doc.Tags = body.Tags
	}
	if body.Notebook != "" {
		doc.Notebook = body.Notebook
	}
	if body.Category != "" {
		doc.Category = body.Category
	}
	doc.UpdatedAt = time.Now()

	if err := s.docRepo.Update(r.Context(), doc); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"id": doc.ID, "title": doc.Title})
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if id == "" {
		writeError(w, 400, "missing id")
		return
	}
	uc := usecase.NewDeleteUseCase(s.docRepo)
	if err := uc.Execute(r.Context(), id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, 400, "missing query param 'q'")
		return
	}

	var tags []string
	if t := r.URL.Query().Get("tags"); t != "" {
		tags = strings.Split(t, ",")
	}

	uc := usecase.NewSearchUseCase(s.docRepo)
	results, err := uc.Execute(r.Context(), usecase.SearchInput{
		Query:    q,
		Tags:     tags,
		Notebook: r.URL.Query().Get("notebook"),
		Category: r.URL.Query().Get("category"),
		Limit:    40,
	})
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	type searchItem struct {
		ID        string   `json:"id"`
		Title     string   `json:"title"`
		Preview   string   `json:"preview"`
		Snippet   string   `json:"snippet"`
		Tags      []string `json:"tags"`
		Notebook  string   `json:"notebook"`
		Category  string   `json:"category"`
		Score     float32  `json:"score"`
		CreatedAt string   `json:"created_at"`
	}

	items := make([]searchItem, len(results))
	for i, r := range results {
		d := r.Document
		preview := strings.ReplaceAll(d.Content, "\n", " ")
		preview = strings.TrimSpace(preview)
		if len(preview) > 120 {
			preview = preview[:117] + "..."
		}
		items[i] = searchItem{
			ID: d.ID, Title: d.Title, Preview: preview,
			Snippet: r.Snippet, Tags: d.Tags,
			Notebook: d.Notebook, Category: d.Category,
			Score:     r.Score,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
		}
	}
	writeJSON(w, 200, items)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	uc := usecase.NewTagsUseCase(s.docRepo)
	tags, err := uc.Execute(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, tags)
}

func (s *Server) handleLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SourceID string `json:"source_id"`
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}
	uc := usecase.NewLinkUseCase(s.docRepo, s.linkRepo)
	if err := uc.Execute(r.Context(), body.SourceID, body.TargetID); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	cmd.Start()
}
