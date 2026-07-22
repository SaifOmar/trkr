package server

import (
	"net/http"

	"github.com/SaifOmar/trkr/store"
	"github.com/SaifOmar/trkr/trkr"
	"github.com/SaifOmar/trkr/types"
)

type Server struct {
	*http.Server
	ActiveSessions       []*types.Session
	ActiveProcesses      []*types.Process
	store                *store.Store
	SUPABASE_URL         string
	SUPABASE_PUBLIC_KEY  string
	SUPABASE_PRIVATE_KEY string
	tr                   *trkr.Traker
}

func New(tr *trkr.Traker, store *store.Store, SUPABASE_URL, SUPABASE_PUBLIC_KEY, SUPABASE_PRIVATE_KEY string, port string) *Server {
	return &Server{
		tr:                   tr,
		Server:               &http.Server{Addr: ":" + port, Handler: nil},
		store:                store,
		SUPABASE_URL:         SUPABASE_URL,
		SUPABASE_PUBLIC_KEY:  SUPABASE_PUBLIC_KEY,
		SUPABASE_PRIVATE_KEY: SUPABASE_PRIVATE_KEY,
		ActiveProcesses:      []*types.Process{},
		ActiveSessions:       []*types.Session{},
	}
}

func (s *Server) JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) json(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		h(w, r)
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/store/processes", s.json(s.GetAllProcesses))
	mux.HandleFunc("GET /api/v1/store/process/{id}", s.json(s.GetProcess))
	mux.HandleFunc("GET /api/v1/store/process", s.json(s.QueryProcesses))

	mux.HandleFunc("GET /api/v1/store/sessions", s.json(s.GetAllSessions))
	mux.HandleFunc("GET /api/v1/store/session/{id}", s.json(s.GetSession))

	mux.HandleFunc("GET /api/v1/store/autowatch", s.json(s.GetAutoWatch))
	mux.HandleFunc("POST /api/v1/store/autowatch", s.json(s.CreateAutoWatch))
	mux.HandleFunc("DELETE /api/v1/store/autowatch/{name}", s.json(s.RemoveAutoWatch))

	mux.HandleFunc("GET /api/v1/active/sessions", s.json(s.GetActiveSessions))
	mux.HandleFunc("POST /api/v1/active/sessions/stop", s.json(s.StopActiveSession))
	mux.HandleFunc("GET /api/v1/active/processes", s.json(s.GetActiveProcesses))
	mux.HandleFunc("GET /api/v1/active/process", s.json(s.QueryActiveProcesses))
	mux.HandleFunc("POST /api/v1/active/processes/watch", s.json(s.StartManualWatch))

	// Serve frontend static files (embedding prefers no-cache so edits reflect on rebuild)
	fs := http.FileServer(staticFS())
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fs.ServeHTTP(w, r)
	}))

	registerPprof(mux)

	s.Server.Handler = mux
	err := s.Server.ListenAndServe()
	if err != nil {
		return err
	}
	return nil
}
