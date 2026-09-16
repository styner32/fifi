package routes

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fifi/internal/dartfiling/config"
	"github.com/fifi/internal/dartfiling/controllers"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// SetupRouter initializes all services, controllers, and API routes
func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	financialController := controllers.FinancialController{DB: db}

	// Set up Gin router
	router := gin.Default()

	// IP-based rate limiting
	var limiters = make(map[string]*clientLimiter)
	var mu sync.Mutex

	// Cleanup stale limiters periodically
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			for ip, client := range limiters {
				if time.Since(client.lastSeen) > 10*time.Minute {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		client, exists := limiters[ip]
		if !exists {
			client = &clientLimiter{
				limiter: rate.NewLimiter(rate.Limit(10), 20), // 10 req/s, burst of 20
			}
			limiters[ip] = client
		}
		client.lastSeen = time.Now()
		return client.limiter
	}

	router.Use(func(c *gin.Context) {
		clientIP := c.ClientIP()
		limiter := getLimiter(clientIP)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			return
		}
		c.Next()
	})

	// Parse allowed origins (trimmed single "*" means open CORS without credentials only)
	allowedOriginsRaw := strings.TrimSpace(cfg.AllowedOrigins)
	wildcardOpen := allowedOriginsRaw == "*"
	var allowedOrigins []string
	if !wildcardOpen {
		for _, o := range strings.Split(allowedOriginsRaw, ",") {
			o = strings.TrimSpace(o)
			if o != "" && o != "*" {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
	}

	// CORS middleware
	router.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if wildcardOpen {
			// * alone: allow any origin via literal * and NEVER credentials.
			// Reflecting arbitrary Origin + credentials would let any site make credentialed requests.
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// Allow-Origin reflects request Origin; caches must partition by Origin or they
			// could serve one origin's CORS headers to another (RFC 9110 / CORS).
			c.Writer.Header().Add("Vary", "Origin")
			allowed := false
			for _, o := range allowedOrigins {
				if o == origin {
					allowed = true
					break
				}
			}
			if allowed && origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Security headers middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Next()
	})

	// Simple health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})

	// Group API routes under /api/v1
	api := router.Group("/api/v1")
	{
		// Companies endpoints
		api.GET("/companies", financialController.GetCompanies)

		// MCP-friendly endpoints
		api.GET("/mcp/reports/by-corp-name", financialController.GetReportsByCorpName)

		// Reports by corp code endpoints
		api.GET("/reports/:corp_code", financialController.GetReportsByCorpCode)

		// Raw reports endpoints
		api.GET("/reports/:corp_code/:raw_report_id", financialController.GetRawReport)

		// Summary + raw report by receipt number
		api.GET("/reports/receipt/:receipt_number", financialController.GetReportSummaryByReceiptNumber)

		// Summary + raw report by receipt number
		api.GET("/mcp/reports/receipt/:receipt_number", financialController.GetReportSummaryByReceiptNumber)

		// Reports endpoints
		api.GET("/reports", financialController.GetAllReports)

		// Reports endpoints
		api.GET("/mcp/reports", financialController.GetAllReports)
	}

	return router
}
