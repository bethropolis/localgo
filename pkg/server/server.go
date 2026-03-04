// Package server provides HTTP server functionality for LocalGo
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// Server manages the HTTP/S server lifecycle.
type Server struct {
	config          *config.Config
	httpServer      *http.Server
	muxRouter       *mux.Router
	receiveService  *services.ReceiveService
	sendService     *services.SendService
	registryService *services.RegistryService
	logger          *zap.SugaredLogger
}

// NewServer creates a new Server instance.
func NewServer(cfg *config.Config, logger *zap.SugaredLogger) *Server {
	httputil.SetLogger(logger)
	router := mux.NewRouter()
	receiveService := services.NewReceiveService()
	sendService := services.NewSendService()
	registryService := services.NewRegistryService()
	return &Server{
		config:          cfg,
		muxRouter:       router,
		receiveService:  receiveService,
		sendService:     sendService,
		registryService: registryService,
		logger:          logger,
	}
}

// configureRoutes sets up the API routes.
func (s *Server) configureRoutes() {
	apiRouter := s.muxRouter.PathPrefix("/api/localsend").Subrouter()

	// Discovery Handlers (Phase 1)
	discoveryHandler := handlers.NewDiscoveryHandler(s.config, s.registryService, s.sendService, s.logger)
	apiRouter.HandleFunc("/v1/info", discoveryHandler.InfoHandler).Methods("GET")
	apiRouter.HandleFunc("/v2/info", discoveryHandler.InfoHandler).Methods("GET")
	apiRouter.HandleFunc("/v1/register", discoveryHandler.RegisterHandler).Methods("POST")
	apiRouter.HandleFunc("/v2/register", discoveryHandler.RegisterHandler).Methods("POST")

	// Receive Handlers (Phase 2)
	receiveHandler := handlers.NewReceiveHandler(s.config, s.receiveService, s.logger)
	apiRouter.HandleFunc("/v1/prepare-upload", receiveHandler.PrepareUploadHandlerV1).Methods("POST")
	apiRouter.HandleFunc("/v2/prepare-upload", receiveHandler.PrepareUploadHandlerV2).Methods("POST")
	apiRouter.HandleFunc("/v2/upload", receiveHandler.UploadHandlerV2).Methods("POST")
	apiRouter.HandleFunc("/v2/cancel", receiveHandler.CancelHandler).Methods("POST")

	// Download Handlers
	downloadHandler := handlers.NewDownloadHandler(s.config, s.sendService, s.logger)
	apiRouter.HandleFunc("/v2/prepare-download", downloadHandler.PrepareDownloadHandler).Methods("POST")
	apiRouter.HandleFunc("/v2/download", downloadHandler.DownloadHandler).Methods("GET")

	s.logger.Info("Configured API routes.")
}

// Start runs the HTTP/S server.
func (s *Server) Start(ctx context.Context, readyChan chan<- struct{}) error {
	s.configureRoutes()

	addr := fmt.Sprintf("0.0.0.0:%d", s.config.Port)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.muxRouter,
		ReadTimeout:       0,
		WriteTimeout:      0,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind port %d: %w", s.config.Port, err)
	}

	serverErrChan := make(chan error, 1)

	if s.config.HttpsEnabled {
		s.logger.Infof("Starting HTTPS server on %s with alias %s", addr, s.config.Alias)
		cert, err := tls.X509KeyPair([]byte(s.config.SecurityContext.Certificate), []byte(s.config.SecurityContext.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to load TLS key pair: %w", err)
		}
		s.httpServer.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		tlsListener := tls.NewListener(ln, s.httpServer.TLSConfig)

		go func() {
			if err := s.httpServer.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
				serverErrChan <- err
			}
		}()
	} else {
		s.logger.Infof("Starting HTTP server on %s with alias %s", addr, s.config.Alias)
		go func() {
			if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
				serverErrChan <- err
			}
		}()
	}

	// Signal that the port is successfully bound
	if readyChan != nil {
		readyChan <- struct{}{}
	}

	// Wait for context cancellation or server error
	select {
	case err := <-serverErrChan:
		return fmt.Errorf("server failed to start: %w", err)
	case <-ctx.Done():
		s.logger.Info("Server shutting down...")
		return s.Shutdown(context.Background())
	}
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
	s.logger.Info("Server stopped.")
	s.httpServer = nil
	return nil
}

// GetSendService returns the SendService instance.
func (s *Server) GetSendService() *services.SendService {
	return s.sendService
}
