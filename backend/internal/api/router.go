package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/konghanghang/openlist-strm/internal/config"
	"github.com/konghanghang/openlist-strm/internal/scheduler"
	"github.com/konghanghang/openlist-strm/internal/storage"
)

// Server represents the API server
type Server struct {
	cfg       *config.Config
	scheduler *scheduler.Scheduler
	db        *storage.DB
	router    *gin.Engine
	auth      *authManager
}

// NewServer creates a new API server
func NewServer(cfg *config.Config, sched *scheduler.Scheduler, db *storage.DB) *Server {
	// Set Gin mode
	if cfg.Log.Level != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	s := &Server{
		cfg:       cfg,
		scheduler: sched,
		db:        db,
		router:    router,
		auth:      newAuthManager(cfg),
	}

	s.setupRoutes()

	return s
}

// setupRoutes sets up all routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", s.handleHealth)

	// Auth routes (public)
	auth := s.router.Group("/api/auth")
	{
		auth.POST("/login", s.handleLogin)
		auth.POST("/logout", s.handleLogout)
		auth.GET("/status", s.handleAuthStatus)
	}

	// Webhook is called by external systems and only accepts the API token
	if s.cfg.API.Token != "" {
		s.router.POST("/api/webhook", s.tokenAuthMiddleware(), s.handleWebhook)
	} else {
		s.router.POST("/api/webhook", s.handleWebhook)
	}

	// Admin API routes: session cookie or API token
	api := s.router.Group("/api")
	{
		api.Use(s.adminAuthMiddleware())

		// Task routes
		api.POST("/generate", s.handleGenerate)
		api.GET("/tasks/:id", s.handleGetTask)
		api.GET("/tasks/:id/logs", s.handleGetTaskLogs)
		api.GET("/tasks", s.handleListTasks)

		// Config routes
		api.GET("/configs", s.handleGetConfigs)
		api.POST("/configs", s.handleCreateMapping)
		api.PUT("/configs/:id", s.handleUpdateMapping)
		api.DELETE("/configs/:id", s.handleDeleteMapping)
		api.GET("/status", s.handleGetStatus)

		// Notification routes
		api.POST("/notify/:id", s.handleNotify)

		// Cron expression preview
		api.POST("/cron/preview", s.handleCronPreview)
	}
}

// Run starts the API server
func (s *Server) Run() error {
	return s.router.Run(s.cfg.GetAddr())
}

// GetRouter returns the gin router
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}
