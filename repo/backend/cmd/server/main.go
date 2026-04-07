package main

import (
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"

	"medops/internal/config"
	"medops/internal/handlers"
	"medops/internal/middleware"
	"medops/internal/repository"
)

func main() {
	cfg := config.Load()

	// Configure logging
	log.SetFormatter(&log.JSONFormatter{})
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)

	log.Info("Starting MedOps Offline Operations Console")

	// Connect to database with retry
	var db *sql.DB
	for i := 0; i < 30; i++ {
		db, err = sql.Open("postgres", cfg.DatabaseURL)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.WithError(err).Warn("Database not ready, retrying in 2s...")
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Info("Connected to database")

	// Run migrations
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.WithError(err).Fatal("Failed to create migration driver")
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+cfg.MigrationsPath, "postgres", driver)
	if err != nil {
		log.WithError(err).Fatal("Failed to create migrator")
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.WithError(err).Fatal("Failed to run migrations")
	}
	log.Info("Database migrations applied")

	// Create data directory
	os.MkdirAll(cfg.DataDir, 0755)

	// Initialize repository (pass encrypt key for at-rest encryption of sensitive member fields)
	repo := repository.New(db, cfg.EncryptKey, cfg.TenantID)

	// Start retention scheduler — purges managed files past their retention_until date.
	retentionDone := make(chan struct{})
	handlers.StartRetentionScheduler(repo, cfg.DataDir, retentionDone)

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(echomw.Recover())
	e.Use(middleware.RequestLogger())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Serve frontend static files
	e.Static("/", "/app/frontend/dist")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(repo, cfg.JWTSecret)
	userHandler := handlers.NewUserHandler(repo)
	inventoryHandler := handlers.NewInventoryHandler(repo)
	learningHandler := handlers.NewLearningHandler(repo)
	workOrderHandler := handlers.NewWorkOrderHandler(repo)
	memberHandler := handlers.NewMemberHandler(repo, cfg.EncryptKey)
	chargeHandler := handlers.NewChargeHandler(repo, cfg.HMACKey)
	fileHandler := handlers.NewFileHandler(repo, cfg.DataDir)
	systemHandler := handlers.NewSystemHandler(repo, cfg.DataDir, cfg.DatabaseURL)

	// Auth middleware
	authMW := middleware.JWTAuth(cfg.JWTSecret)
	adminRole := middleware.RequireRole("system_admin")
	inventoryRole := middleware.RequireRole("system_admin", "inventory_pharmacist")
	learningRole := middleware.RequireRole("system_admin", "learning_coordinator")
	frontDeskRole := middleware.RequireRole("system_admin", "front_desk")
	maintenanceRole := middleware.RequireRole("system_admin", "maintenance_tech")

	// API routes
	api := e.Group("/api/v1")

	// Health (no auth)
	api.GET("/health", systemHandler.HealthCheck)

	// Auth (no auth required for login)
	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/logout", authHandler.Logout, authMW)
	api.GET("/auth/me", authHandler.GetMe, authMW)
	api.PUT("/auth/password", authHandler.ChangePassword, authMW)

	// Users (admin only)
	users := api.Group("/users", authMW, adminRole)
	users.GET("", userHandler.ListUsers)
	users.POST("", userHandler.CreateUser)
	users.PUT("/:id", userHandler.UpdateUser)
	users.DELETE("/:id", userHandler.DeleteUser)
	users.POST("/:id/unlock", userHandler.UnlockUser)

	// SKUs (inventory role)
	skus := api.Group("/skus", authMW, inventoryRole)
	skus.GET("", inventoryHandler.ListSKUs)
	skus.POST("", inventoryHandler.CreateSKU)
	skus.GET("/low-stock", inventoryHandler.GetLowStock)
	skus.GET("/:id", inventoryHandler.GetSKU)
	skus.PUT("/:id", inventoryHandler.UpdateSKU)
	skus.GET("/:id/batches", inventoryHandler.GetBatches)

	// Inventory transactions (inventory role)
	inv := api.Group("/inventory", authMW, inventoryRole)
	inv.POST("/receive", inventoryHandler.Receive)
	inv.POST("/dispense", inventoryHandler.Dispense)
	inv.GET("/transactions", inventoryHandler.ListTransactions)
	inv.POST("/adjust", inventoryHandler.Adjust)

	// Stocktakes (inventory role)
	st := api.Group("/stocktakes", authMW, inventoryRole)
	st.POST("", inventoryHandler.CreateStocktake)
	st.GET("/:id", inventoryHandler.GetStocktake)
	st.PUT("/:id/lines", inventoryHandler.UpdateStocktakeLines)
	st.POST("/:id/complete", inventoryHandler.CompleteStocktake)

	// Learning (read: any auth, write: learning role)
	learn := api.Group("/learning", authMW)
	learn.GET("/subjects", learningHandler.ListSubjects)
	learn.POST("/subjects", learningHandler.CreateSubject, learningRole)
	learn.PUT("/subjects/:id", learningHandler.UpdateSubject, learningRole)
	learn.GET("/chapters", learningHandler.ListChapters)
	learn.POST("/chapters", learningHandler.CreateChapter, learningRole)
	learn.GET("/knowledge-points", learningHandler.ListKnowledgePoints)
	learn.POST("/knowledge-points", learningHandler.CreateKnowledgePoint, learningRole)
	learn.PUT("/knowledge-points/:id", learningHandler.UpdateKnowledgePoint, learningRole)
	learn.GET("/search", learningHandler.SearchKnowledgePoints)
	learn.POST("/import", learningHandler.ImportContent, learningRole)
	learn.GET("/export/:id", learningHandler.ExportContent)

	// Work Orders (any auth can submit, maintenance manages)
	wo := api.Group("/work-orders", authMW)
	wo.GET("", workOrderHandler.ListWorkOrders)
	wo.POST("", workOrderHandler.CreateWorkOrder)
	wo.GET("/analytics", workOrderHandler.GetAnalytics, maintenanceRole)
	wo.GET("/:id", workOrderHandler.GetWorkOrder)
	wo.PUT("/:id", workOrderHandler.UpdateWorkOrder, maintenanceRole)
	wo.POST("/:id/close", workOrderHandler.CloseWorkOrder, maintenanceRole)
	wo.POST("/:id/rate", workOrderHandler.RateWorkOrder)

	// Members (front desk role)
	mem := api.Group("/members", authMW, frontDeskRole)
	mem.GET("", memberHandler.ListMembers)
	mem.POST("", memberHandler.CreateMember)
	mem.GET("/:id", memberHandler.GetMember)
	mem.PUT("/:id", memberHandler.UpdateMember)
	mem.POST("/:id/freeze", memberHandler.FreezeMember)
	mem.POST("/:id/unfreeze", memberHandler.UnfreezeMember)
	mem.POST("/:id/redeem", memberHandler.RedeemBenefit)
	mem.POST("/:id/add-value", memberHandler.AddValue)
	mem.POST("/:id/refund", memberHandler.RefundStoredValue)
	mem.GET("/:id/transactions", memberHandler.ListTransactions)
	api.GET("/membership-tiers", memberHandler.ListTiers, authMW, frontDeskRole)

	// Rate Tables & Charges (admin role)
	rt := api.Group("/rate-tables", authMW, adminRole)
	rt.GET("", chargeHandler.ListRateTables)
	rt.POST("", chargeHandler.CreateRateTable)
	rt.PUT("/:id", chargeHandler.UpdateRateTable)
	rt.POST("/import-csv", chargeHandler.ImportRateTableCSV)

	stmts := api.Group("/statements", authMW, adminRole)
	stmts.GET("", chargeHandler.ListStatements)
	stmts.POST("/generate", chargeHandler.GenerateStatement)
	stmts.GET("/:id", chargeHandler.GetStatement)
	stmts.POST("/:id/reconcile", chargeHandler.ReconcileStatement)
	stmts.POST("/:id/approve", chargeHandler.ApproveStatement)
	stmts.POST("/:id/export", chargeHandler.ExportStatement)

	// Files (any auth)
	files := api.Group("/files", authMW)
	files.POST("/upload", fileHandler.Upload)
	files.GET("/:id", fileHandler.Download)
	files.POST("/export-zip", fileHandler.ExportZip)

	// System (admin)
	sys := api.Group("/system", authMW, adminRole)
	sys.POST("/backup", systemHandler.Backup)
	sys.GET("/backup/status", systemHandler.BackupStatus)
	sys.POST("/update", systemHandler.ApplyUpdate)
	sys.POST("/rollback", systemHandler.Rollback)
	sys.GET("/config", systemHandler.GetConfig)
	sys.PUT("/config", systemHandler.UpdateConfig)

	// Drafts (any auth)
	drafts := api.Group("/drafts", authMW)
	drafts.GET("", systemHandler.ListDrafts)
	drafts.PUT("/:formType", systemHandler.SaveDraft)
	drafts.GET("/:formType/:formId", systemHandler.GetDraft)
	drafts.DELETE("/:formType/:formId", systemHandler.DeleteDraft)

	// SPA fallback — serve index.html for non-API, non-static routes
	e.GET("/*", func(c echo.Context) error {
		return c.File("/app/frontend/dist/index.html")
	})

	// Graceful shutdown
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")
	close(retentionDone)
	e.Close()
	db.Close()
	log.Info("Server stopped")
}

func init() {
	// Ensure correct timezone
	time.Local = time.UTC
}
