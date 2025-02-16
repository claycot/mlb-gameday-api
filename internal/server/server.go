package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/claycot/mlb-gameday-api/internal/config"
	"github.com/rs/cors"
)

type Server struct {
	addr    string
	handler http.Handler
	logger  *log.Logger
}

func New(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, logger *log.Logger) (*Server, error) {
	// initialize routes, passing wg for worker daemons
	router := Initialize(ctx, wg, logger)

	// configure CORS usuing the config
	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedMethods: []string{"GET"},
		AllowedHeaders: []string{"Content-Type"},
	})

	return &Server{
		addr:    fmt.Sprintf("%s:%d", cfg.Hostname, cfg.Port),
		handler: corsMiddleware.Handler(router),
		logger:  logger,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	s.logger.Printf("Starting server on %s", s.addr)

	server := &http.Server{
		Addr:    s.addr,
		Handler: s.handler,
	}

	go func() {
		<-ctx.Done()
		s.logger.Println("Shutting down server...")
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctxShutdown); err != nil {
			s.logger.Printf("Error during server shutdown: %v", err)
		}
	}()

	return server.ListenAndServe()
}
