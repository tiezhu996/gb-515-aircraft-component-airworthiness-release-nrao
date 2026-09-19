package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/config"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/handler"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/middleware"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/repository"
	"github.com/blueship581/aircraft-component-airworthiness-release/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	aircraftPartRepository := repository.NewAircraftPartRepository(db)
	inspectionTaskRepository := repository.NewInspectionTaskRepository(db)
	certificateRecordRepository := repository.NewCertificateRecordRepository(db)
	releaseAuthorizationRepository := repository.NewReleaseAuthorizationRepository(db)
	aircraftPartService := service.NewAircraftPartService(aircraftPartRepository, securityService)
	inspectionTaskService := service.NewInspectionTaskService(inspectionTaskRepository, securityService)
	certificateRecordService := service.NewCertificateRecordService(certificateRecordRepository, securityService)
	releaseAuthorizationService := service.NewReleaseAuthorizationService(releaseAuthorizationRepository, securityService)
	aircraftPartHandler := handler.NewAircraftPartHandler(aircraftPartService)
	inspectionTaskHandler := handler.NewInspectionTaskHandler(inspectionTaskService)
	certificateRecordHandler := handler.NewCertificateRecordHandler(certificateRecordService)
	releaseAuthorizationHandler := handler.NewReleaseAuthorizationHandler(releaseAuthorizationService)
	systemHandler := handler.NewSystemHandler(securityService, aircraftPartService, inspectionTaskService, certificateRecordService, releaseAuthorizationService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	engine.POST("/api/auth/login", systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", systemHandler.Runtime)
	api.GET("/audit-summary", systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", systemHandler.EntityHistory)
	aircraftPartHandler.Register(api)
	inspectionTaskHandler.Register(api)
	certificateRecordHandler.Register(api)
	releaseAuthorizationHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
