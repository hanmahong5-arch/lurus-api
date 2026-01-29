package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/lurus-api/internal/biz/service"
	"github.com/QuantumNous/lurus-api/internal/data/model"
	"github.com/QuantumNous/lurus-api/internal/pkg/common"
	"github.com/QuantumNous/lurus-api/internal/pkg/constant"
	"github.com/QuantumNous/lurus-api/internal/pkg/logger"
	"github.com/QuantumNous/lurus-api/internal/pkg/search"
	"github.com/QuantumNous/lurus-api/internal/pkg/setting/ratio_setting"
	"github.com/QuantumNous/lurus-api/internal/server/controller"
	"github.com/QuantumNous/lurus-api/internal/server/middleware"
	"github.com/QuantumNous/lurus-api/internal/server/router"
	"github.com/QuantumNous/lurus-api/web"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/sync/errgroup"

	_ "net/http/pprof"
)

var buildFS = web.BuildFS
var indexPage = web.IndexPage

func main() {
	startTime := time.Now()

	// Create root context with signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, startTime); err != nil && !errors.Is(err, context.Canceled) {
		common.FatalLog("server error: " + err.Error())
		os.Exit(1)
	}

	common.SysLog("server shutdown complete")
}

func run(ctx context.Context, startTime time.Time) error {
	if err := InitResources(); err != nil {
		return fmt.Errorf("failed to initialize resources: %w", err)
	}

	common.SysLog("New API " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	// Ensure database is closed on shutdown
	defer func() {
		if err := model.CloseDB(); err != nil {
			common.SysError("failed to close database: " + err.Error())
		}
	}()

	// Use errgroup for managing background goroutines with context cancellation
	g, ctx := errgroup.WithContext(ctx)

	if common.RedisEnabled {
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Initialize channel cache with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					if _, _, fixErr := model.FixAbility(); fixErr != nil {
						common.SysError(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		// Background task: sync channel cache
		g.Go(func() error {
			model.SyncChannelCacheWithContext(ctx, common.SyncFrequency)
			return nil
		})
	}

	// Background task: sync options (hot reload config)
	g.Go(func() error {
		model.SyncOptionsWithContext(ctx, common.SyncFrequency)
		return nil
	})

	// Background task: update quota dashboard data
	g.Go(func() error {
		model.UpdateQuotaDataWithContext(ctx)
		return nil
	})

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			return fmt.Errorf("failed to parse CHANNEL_UPDATE_FREQUENCY: %w", err)
		}
		g.Go(func() error {
			controller.AutomaticallyUpdateChannelsWithContext(ctx, frequency)
			return nil
		})
	}

	// Background task: automatically test channels
	g.Go(func() error {
		controller.AutomaticallyTestChannelsWithContext(ctx)
		return nil
	})

	if common.IsMasterNode && constant.UpdateTask {
		g.Go(func() error {
			controller.UpdateMidjourneyTaskBulkWithContext(ctx)
			return nil
		})
		g.Go(func() error {
			controller.UpdateTaskBulkWithContext(ctx)
			return nil
		})
	}

	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}

	// Start daily quota reset cron job
	if os.Getenv("DAILY_QUOTA_ENABLED") != "false" {
		model.StartDailyQuotaResetCron()
	}

	// pprof server
	if os.Getenv("ENABLE_PPROF") == "true" {
		pprofServer := &http.Server{Addr: "0.0.0.0:8005", Handler: nil}
		g.Go(func() error {
			common.SysLog("pprof enabled on :8005")
			if err := pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("pprof server error: %w", err)
			}
			return nil
		})
		g.Go(func() error {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return pprofServer.Shutdown(shutdownCtx)
		})
		g.Go(func() error {
			common.MonitorWithContext(ctx)
			return nil
		})
	}

	if err := common.StartPyroScope(); err != nil {
		common.SysError(fmt.Sprintf("start pyroscope error: %v", err))
	}

	// Initialize Gin HTTP server
	engine := gin.New()
	engine.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please submit an issue.", err),
				"type":    "new_api_panic",
			},
		})
	}))
	engine.Use(middleware.RequestId())
	middleware.SetUpLogger(engine)

	// Initialize session store
	store := cookie.NewStore([]byte(common.SessionSecret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   7776000, // 90 days
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
	engine.Use(sessions.Sessions("session", store))

	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	// Setup routes
	router.SetRouter(engine, buildFS, indexPage)

	port := os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	// Create http.Server for graceful shutdown
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: engine,
	}

	// Start HTTP server
	g.Go(func() error {
		common.LogStartupSuccess(startTime, port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server error: %w", err)
		}
		return nil
	})

	// Graceful shutdown handler
	g.Go(func() error {
		<-ctx.Done()
		common.SysLog("shutdown signal received, initiating graceful shutdown...")

		// Allow 30 seconds for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP server shutdown error: %w", err)
		}
		common.SysLog("HTTP server shutdown complete")
		return nil
	})

	// Wait for all goroutines to complete
	return g.Wait()
}

func InjectUmamiAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("UMAMI_WEBSITE_ID") != "" {
		umamiSiteID := os.Getenv("UMAMI_WEBSITE_ID")
		umamiScriptURL := os.Getenv("UMAMI_SCRIPT_URL")
		if umamiScriptURL == "" {
			umamiScriptURL = "https://analytics.umami.is/script.js"
		}
		analyticsInjectBuilder.WriteString("<script defer src=\"")
		analyticsInjectBuilder.WriteString(umamiScriptURL)
		analyticsInjectBuilder.WriteString("\" data-website-id=\"")
		analyticsInjectBuilder.WriteString(umamiSiteID)
		analyticsInjectBuilder.WriteString("\"></script>")
	}
	analyticsInjectBuilder.WriteString("<!--Umami QuantumNous-->\n")
	analyticsInject := analyticsInjectBuilder.String()
	indexPage = bytes.ReplaceAll(indexPage, []byte("<!--umami-->\n"), []byte(analyticsInject))
}

func InjectGoogleAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("GOOGLE_ANALYTICS_ID") != "" {
		gaID := os.Getenv("GOOGLE_ANALYTICS_ID")
		// Google Analytics 4 (gtag.js)
		analyticsInjectBuilder.WriteString("<script async src=\"https://www.googletagmanager.com/gtag/js?id=")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("\"></script>")
		analyticsInjectBuilder.WriteString("<script>")
		analyticsInjectBuilder.WriteString("window.dataLayer = window.dataLayer || [];")
		analyticsInjectBuilder.WriteString("function gtag(){dataLayer.push(arguments);}")
		analyticsInjectBuilder.WriteString("gtag('js', new Date());")
		analyticsInjectBuilder.WriteString("gtag('config', '")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("');")
		analyticsInjectBuilder.WriteString("</script>")
	}
	analyticsInjectBuilder.WriteString("<!--Google Analytics QuantumNous-->\n")
	analyticsInject := analyticsInjectBuilder.String()
	indexPage = bytes.ReplaceAll(indexPage, []byte("<!--Google Analytics-->\n"), []byte(analyticsInject))
}

func InitResources() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	common.InitEnv()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	model.InitOptionMap()

	// 初始化模型
	model.GetPricing()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}

	// Initialize Meilisearch
	// 初始化 Meilisearch
	err = search.InitMeilisearch()
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to initialize Meilisearch: %v", err))
		// Don't return error - Meilisearch is optional
		// 不返回错误 - Meilisearch 是可选的
	} else if search.IsEnabled() {
		// Initialize search sync mechanism
		// 初始化搜索同步机制
		err = search.InitSync()
		if err != nil {
			common.SysError(fmt.Sprintf("Failed to initialize Meilisearch sync: %v", err))
		}
	}

	// Initialize subscription plans
	// 初始化订阅计划
	model.InitSubscriptionPlans()

	// Start subscription cron jobs
	// 启动订阅定时任务
	model.StartSubscriptionCronJobs()

	// Initialize Zitadel authentication (multi-tenant OAuth)
	// 初始化 Zitadel 认证（多租户 OAuth）
	err = middleware.InitZitadelAuth()
	if err != nil {
		common.SysError(fmt.Sprintf("Failed to initialize Zitadel authentication: %v", err))
		// Don't return error - Zitadel is optional and can be enabled later
		// 不返回错误 - Zitadel 是可选的，可以稍后启用
	}

	return nil
}
