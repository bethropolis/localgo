// Package server provides HTTP server functionality for LocalGo
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Server manages the HTTP/S server lifecycle.
type Server struct {
	config         *config.Config
	httpServer     *http.Server
	muxRouter      *mux.Router
	receiveService *services.ReceiveService
	sendService    *services.SendService
}

// NewServer creates a new Server instance.
func NewServer(cfg *config.Config) *Server {
	router := mux.NewRouter()
	receiveService := services.NewReceiveService()
	sendService := services.NewSendService()
	return &Server{
		config:         cfg,
		muxRouter:      router,
		receiveService: receiveService,
		sendService:    sendService,
	}
}

// configureRoutes sets up the API routes.
func (s *Server) configureRoutes() {
	apiRouter := s.muxRouter.PathPrefix("/api/localsend").Subrouter()

	// Discovery Handlers (Phase 1)
	discoveryHandler := handlers.NewDiscoveryHandler(s.config)
	apiRouter.HandleFunc("/v1/info", discoveryHandler.InfoHandler).Methods("GET")
	apiRouter.HandleFunc("/v2/info", discoveryHandler.InfoHandler).Methods("GET")
	apiRouter.HandleFunc("/v1/register", discoveryHandler.RegisterHandler).Methods("POST")
	apiRouter.HandleFunc("/v2/register", discoveryHandler.RegisterHandler).Methods("POST")

	// Receive Handlers (Phase 2)
	receiveHandler := handlers.NewReceiveHandler(s.config, s.receiveService)
	apiRouter.HandleFunc("/v1/prepare-upload", receiveHandler.PrepareUploadHandlerV1).Methods("POST")
	apiRouter.HandleFunc("/v2/prepare-upload", receiveHandler.PrepareUploadHandlerV2).Methods("POST")
	apiRouter.HandleFunc("/v2/upload", receiveHandler.UploadHandlerV2).Methods("POST")
	apiRouter.HandleFunc("/v2/cancel", receiveHandler.CancelHandler).Methods("POST")

	// Download Handlers
	downloadHandler := handlers.NewDownloadHandler(s.config, s.sendService)
	apiRouter.HandleFunc("/v2/prepare-download", downloadHandler.PrepareDownloadHandler).Methods("POST")
	apiRouter.HandleFunc("/v2/download", downloadHandler.DownloadHandler).Methods("GET")

	logrus.Info("Configured API routes.")
}

// Start runs the HTTP/S server.
func (s *Server) Start(ctx context.Context) error {
	s.configureRoutes()

	addr := fmt.Sprintf("0.0.0.0:%d", s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.muxRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if s.config.HttpsEnabled {
		logrus.Infof("Starting HTTPS server on %s with alias %s", addr, s.config.Alias)
		cert, err := tls.X509KeyPair([]byte(s.config.SecurityContext.Certificate), []byte(s.config.SecurityContext.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to load TLS key pair: %w", err)
		}
		s.httpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		go func() {
			if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logrus.Fatalf("HTTPS server failed: %v", err)
			}
		}()
	} else {
		logrus.Infof("Starting HTTP server on %s with alias %s", addr, s.config.Alias)
		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logrus.Fatalf("HTTP server failed: %v", err)
			}
		}()
	}

	<-ctx.Done()
	logrus.Info("Server shutting down...")
	return s.Shutdown(context.Background())
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	logrus.Info("Server stopped.")
	s.httpServer = nil
	return nil
}
