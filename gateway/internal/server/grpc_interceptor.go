package server

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ratelimiter/gateway/pkg/models"
)

type GrpcInterceptor struct {
	coordinator *Coordinator
}

func NewGrpcInterceptor(c *Coordinator) *GrpcInterceptor {
	return &GrpcInterceptor{coordinator: c}
}

func (gi *GrpcInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)

		reqID := getHeader(md, "x-request-id")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		headers := make(map[string]string)
		for k, v := range md {
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

		methodPath := info.FullMethod
		methodParts := strings.SplitN(methodPath, "/", 3)
		method := "RPC"
		if len(methodParts) >= 2 {
			method = methodParts[1]
		}

		reqCtx := &models.RequestContext{
			RequestID:  reqID,
			APIPath:    methodPath,
			Method:     method,
			UserID:     getHeader(md, "x-user-id"),
			TenantID:   getHeader(md, "x-tenant-id"),
			ClientIP:   getHeader(md, "x-real-ip"),
			Headers:    headers,
			Priority:   priority,
			ReceivedAt: time.Now(),
		}

		result, err := gi.coordinator.Process(ctx, reqCtx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "rate limit error: %v", err)
		}

		trailerMD := metadata.Pairs(
			"x-ratelimit-limit", strconv.FormatInt(min64(result.Limit, 1000000), 10),
			"x-ratelimit-remaining", strconv.FormatInt(result.Remaining, 10),
			"x-ratelimit-reset", strconv.FormatInt(result.ResetTime.Unix(), 10),
			"x-ratelimit-mode", string(result.Mode),
		)
		grpc.SetTrailer(ctx, trailerMD)

		if !result.Allowed {
			retryMD := metadata.Pairs(
				"retry-after", strconv.FormatInt(result.RetryAfter, 10),
			)
			grpc.SetTrailer(ctx, retryMD)
			return nil, status.Errorf(
				codes.ResourceExhausted,
				"rate limit exceeded: rule=%s, retry_after=%ds, triggered=%s",
				result.RuleID, result.RetryAfter, result.TriggeredLevel,
			)
		}

		if result.Queued && result.QueueDelayMs > 0 {
			time.Sleep(time.Duration(result.QueueDelayMs) * time.Millisecond)
		}

		start := time.Now()
		resp, handlerErr := handler(ctx, req)
		latency := time.Since(start)
		gi.coordinator.RecordBackendMetrics(
			int64(latency.Milliseconds()),
			handlerErr != nil && status.Code(handlerErr) >= codes.Internal,
		)
		return resp, handlerErr
	}
}

func (gi *GrpcInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)

		reqID := getHeader(md, "x-request-id")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		headers := make(map[string]string)
		for k, v := range md {
			if len(v) > 0 {
				headers[strings.ToLower(k)] = strings.ToLower(v[0])
			}
		}

		reqCtx := &models.RequestContext{
			RequestID:  reqID,
			APIPath:    info.FullMethod,
			Method:     "STREAM",
			UserID:     getHeader(md, "x-user-id"),
			TenantID:   getHeader(md, "x-tenant-id"),
			ClientIP:   getHeader(md, "x-real-ip"),
			Headers:    headers,
			Priority:   0,
			ReceivedAt: time.Now(),
		}

		result, err := gi.coordinator.Process(ctx, reqCtx)
		if err != nil {
			return status.Errorf(codes.Internal, "rate limit error: %v", err)
		}

		trailerMD := metadata.Pairs(
			"x-ratelimit-limit", strconv.FormatInt(min64(result.Limit, 1000000), 10),
			"x-ratelimit-remaining", strconv.FormatInt(result.Remaining, 10),
			"x-ratelimit-reset", strconv.FormatInt(result.ResetTime.Unix(), 10),
		)
		ss.SetTrailer(trailerMD)

		if !result.Allowed {
			return status.Errorf(
				codes.ResourceExhausted,
				"rate limit exceeded: rule=%s, retry_after=%ds",
				result.RuleID, result.RetryAfter,
			)
		}

		if result.Queued && result.QueueDelayMs > 0 {
			time.Sleep(time.Duration(result.QueueDelayMs) * time.Millisecond)
		}

		return handler(srv, ss)
	}
}

func getHeader(md metadata.MD, key string) string {
	vals := md.Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
