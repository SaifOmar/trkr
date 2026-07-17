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
		Server:               &http.Server{Addr: port, Handler: nil},
		store:                store,
		SUPABASE_URL:         SUPABASE_URL,
		SUPABASE_PUBLIC_KEY:  SUPABASE_PUBLIC_KEY,
		SUPABASE_PRIVATE_KEY: SUPABASE_PRIVATE_KEY,
	}
}
func (s *Server) JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/store/processes", s.GetAllProcesses)
	mux.HandleFunc("GET /api/v1/store/process/{id}", s.GetProcess)
	mux.HandleFunc("GET /api/v1/store/sessions", s.GetAllSessions)
	mux.HandleFunc("GET /api/v1/store/session/{id}", s.GetSession)

	mux.HandleFunc("GET /api/v1/active/sessions", s.GetCurrentSessions)
	mux.HandleFunc("GET /api/v1/active/processes", s.GetCurrentProcesses)

	mux.HandleFunc("GET /api/v1/store/autowatch", s.GetAutoWatch)
	mux.HandleFunc("POST /api/v1/store/autowatch", s.CreateAutoWatch)
	mux.HandleFunc("DELETE /api/v1/store/autowatch/{name}", s.RemoveAutoWatch)

	handler := s.JSONMiddleware(mux)
	s.Server.Handler = handler
	err := s.Server.ListenAndServe()
	if err != nil {
		return err
	}
	return nil
}
