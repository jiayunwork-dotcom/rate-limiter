package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ratelimiter/gateway/internal/metrics"
	"github.com/ratelimiter/gateway/pkg/models"
)

type GatewayServer struct {
	engine      *gin.Engine
	coordinator *Coordinator
	metrics     *metrics.Collector
	targetURL   *url.URL
	proxy       *httputil.ReverseProxy
	httpServer  *http.Server
	nodeID      string
}

func NewGatewayServer(
	nodeID string,
	coordinator *Coordinator,
	collector *metrics.Collector,
	targetAddr string,
) (*GatewayServer, error) {

	target, err := url.Parse(targetAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
	}

	gs := &GatewayServer{
		engine:      engine,
		coordinator: coordinator,
		metrics:     collector,
		targetURL:   target,
		proxy:       proxy,
		nodeID:      nodeID,
	}
	gs.setupRoutes()
	return gs, nil
}

func (gs *GatewayServer) setupRoutes() {
	gs.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	gs.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"node":   gs.nodeID,
			"mode":   gs.coordinator.GetMode(),
			"time":   time.Now().UTC(),
		})
	})

	gs.engine.GET("/health/adaptive", func(c *gin.Context) {
		c.JSON(200, gs.coordinator.GetHealth())
	})

	gs.engine.NoRoute(gs.rateLimitMiddleware(), func(c *gin.Context) {
		start := time.Now()
		gs.proxy.ServeHTTP(c.Writer, c.Request)
		latency := time.Since(start)
		gs.metrics.RecordLatency(c.Request.URL.Path, c.Request.Method, latency)
		gs.coordinator.RecordBackendMetrics(
			int64(latency.Milliseconds()),
			c.Writer.Status() >= 500,
		)
	})
}

func (gs *GatewayServer) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		headers := make(map[string]string)
		for k, v := range c.Request.Header {
			if len(v) > 0 {
				headers[strings.ToLower(k)] = strings.ToLower(v[0])
			}
		}

		priority := 0
		if p, ok := headers["x-priority"]; ok {
			if pv, err := strconv.Atoi(p); err == nil {
				priority = pv
			}
		}

		clientIP := c.ClientIP()
		if fwd := headers["x-forwarded-for"]; fwd != "" {
			ips := strings.Split(fwd, ",")
			if len(ips) > 0 {
				clientIP = strings.TrimSpace(ips[0])
			}
		}

		reqCtx := &models.RequestContext{
			RequestID:  reqID,
			APIPath:    c.Request.URL.Path,
			Method:     c.Request.Method,
			UserID:     headers["x-user-id"],
			TenantID:   headers["x-tenant-id"],
			ClientIP:   clientIP,
			Headers:    headers,
			Priority:   priority,
			ReceivedAt: time.Now(),
		}

		result, err := gs.coordinator.Process(c.Request.Context(), reqCtx)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "rate limit service error"})
			return
		}

		gs.setRateLimitHeaders(c, result)

		status := "200"
		if !result.Allowed {
			status = "429"
		}
		gs.metrics.RecordRequest(
			reqCtx.TenantID, reqCtx.UserID, reqCtx.APIPath,
			reqCtx.Method, status,
		)

		if result.Queued && result.QueueDelayMs > 0 {
			time.Sleep(time.Duration(result.QueueDelayMs) * time.Millisecond)
		}

		if !result.Allowed {
			c.Header("Retry-After", strconv.FormatInt(result.RetryAfter, 10))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":         "rate limit exceeded",
				"rule_id":       result.RuleID,
				"retry_after":   result.RetryAfter,
				"triggered_by":  string(result.TriggeredLevel),
				"mode":          string(result.Mode),
			})
			return
		}

		c.Next()
	}
}

func (gs *GatewayServer) setRateLimitHeaders(c *gin.Context, r *models.RateLimitResult) {
	limit := r.Limit
	if limit > 1000000 {
		limit = 1000000
	}
	c.Header("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
	c.Header("X-RateLimit-Remaining", strconv.FormatInt(r.Remaining, 10))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(r.ResetTime.Unix(), 10))
	c.Header("X-RateLimit-Mode", string(r.Mode))
	c.Header("X-RateLimit-Node", gs.nodeID)
}

func (gs *GatewayServer) Start(addr string) error {
	gs.httpServer = &http.Server{
		Addr:         addr,
		Handler:      gs.engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return gs.httpServer.ListenAndServe()
}

func (gs *GatewayServer) Shutdown(ctx context.Context) error {
	if gs.httpServer != nil {
		return gs.httpServer.Shutdown(ctx)
	}
	return nil
}
