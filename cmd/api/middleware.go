package main

import (
	"context"
	"errors"
	"expvar"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/shadyar-bakr/greenlight/internal/data"
	"github.com/shadyar-bakr/greenlight/internal/validator"
	"golang.org/x/time/rate"
)

const (
	traceIDContextKey    = contextKey("trace_id")
	startTimeContextKey  = contextKey("start_time")
	userAgentContextKey  = contextKey("user_agent")
	remoteAddrContextKey = contextKey("remote_addr")
)

type permissionCache struct {
	permissions data.Permissions
	expiry      time.Time
}

func (app *application) metrics(next http.Handler) http.Handler {
	var (
		totalRequestsReceived           = expvar.NewInt("total_requests_received")
		totalResponsesSent              = expvar.NewInt("total_responses_sent")
		totalProcessingTimeMicroseconds = expvar.NewInt("total_processing_time_μs")
		totalResponsesSentByStatus      = expvar.NewMap("total_responses_sent_by_status")
		totalRequestsByPath             = expvar.NewMap("total_requests_by_path")
		totalRequestsByMethod           = expvar.NewMap("total_requests_by_method")
		activeConnections               = expvar.NewInt("active_connections")
		totalResponseSize               = expvar.NewInt("total_response_size_bytes")
		averageResponseTime             = expvar.NewFloat("average_response_time_ms")
		requestCount                    = expvar.NewInt("request_count")
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		activeConnections.Add(1)
		defer activeConnections.Add(-1)

		// Record request metrics
		totalRequestsReceived.Add(1)
		totalRequestsByPath.Add(r.URL.Path, 1)
		totalRequestsByMethod.Add(r.Method, 1)

		// Use Chi's middleware.WrapResponseWriter with additional tracking
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		// Record response metrics
		totalResponsesSent.Add(1)
		totalResponsesSentByStatus.Add(strconv.Itoa(ww.Status()), 1)
		totalResponseSize.Add(int64(ww.BytesWritten()))

		// Calculate and update timing metrics
		duration := time.Since(start)
		durationMS := float64(duration) / float64(time.Millisecond)
		totalProcessingTimeMicroseconds.Add(duration.Microseconds())

		// Update average response time
		oldCount := requestCount.Value()
		requestCount.Add(1)
		newAvg := (averageResponseTime.Value()*float64(oldCount) + durationMS) / float64(oldCount+1)
		averageResponseTime.Set(newAvg)

		// Log request details with additional information
		app.logger.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", durationMS,
			"size_bytes", ww.BytesWritten(),
			"request_id", middleware.GetReqID(r.Context()),
			"client_ip", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	type client struct {
		limiter *rate.Limiter

		lastSeen time.Time
	}

	var clients sync.Map

	// Background cleanup using ticker
	go func() {
		ticker := time.NewTicker(app.config.limiter.cleanup)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			clients.Range(func(key, value interface{}) bool {
				client := value.(*client)
				if now.Sub(client.lastSeen) > app.config.limiter.cleanup {
					clients.Delete(key)
				}
				return true
			})
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !app.config.limiter.enabled {
			next.ServeHTTP(w, r)
			return
		}

		ip := middleware.GetReqID(r.Context())
		if ip == "" {
			ip = r.RemoteAddr
		}

		value, _ := clients.LoadOrStore(ip, &client{
			limiter:  rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst),
			lastSeen: time.Now(),
		})

		currentClient := value.(*client)
		currentClient.lastSeen = time.Now()

		if !currentClient.limiter.Allow() {
			app.rateLimitExceededResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add multiple Vary headers
		w.Header().Add("Vary", "Authorization")
		w.Header().Add("Vary", "Accept")

		// Get the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// Validate Bearer token format
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// Extract token without the prefix
		token := authHeader[len(prefix):]

		// Validate token format
		v := validator.New()
		if data.ValidateTokenPlaintext(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		// Get user with a context timeout
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		user, err := app.models.Users.GetForToken(ctx, data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			case errors.Is(err, context.DeadlineExceeded):
				app.serverErrorResponse(w, r, errors.New("authentication timed out"))
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

func (app *application) requirePermission(code string) func(http.Handler) http.Handler {
	var cache sync.Map
	const cacheDuration = 5 * time.Minute

	return func(next http.Handler) http.Handler {
		fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := app.contextGetUser(r)

			// Check cache first
			if cached, ok := cache.Load(user.ID); ok {
				if pc := cached.(permissionCache); time.Now().Before(pc.expiry) {
					if !pc.permissions.Include(code) {
						app.notPermittedResponse(w, r)
						return
					}
					next.ServeHTTP(w, r)
					return
				}
				cache.Delete(user.ID) // Cache expired, remove it
			}

			// Cache miss or expired, fetch from database
			permissions, err := app.models.Permissions.GetAllForUser(user.ID)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}

			// Update cache with expiry
			cache.Store(user.ID, permissionCache{
				permissions: permissions,
				expiry:      time.Now().Add(cacheDuration),
			})

			if !permissions.Include(code) {
				app.notPermittedResponse(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})

		return app.requireActivatedUser(fn)
	}
}

func (app *application) requireActivatedUser(next http.Handler) http.Handler {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	return app.requireAuthenticatedUser(fn)
}

func (app *application) requireAuthenticatedUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) validateRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request size for non-GET/HEAD requests
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.ContentLength > 1_048_576 { // 1MB
				app.errorResponse(w, r, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
		}

		// Validate Accept header for API requests
		if strings.HasPrefix(r.URL.Path, "/v1") {
			accept := r.Header.Get("Accept")
			if accept != "" && accept != "*/*" && !strings.Contains(accept, "application/json") {
				app.errorResponse(w, r, http.StatusNotAcceptable, "content type not acceptable")
				return
			}
		}

		// Validate Content-Type for requests with bodies
		if r.ContentLength > 0 {
			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" {
				app.errorResponse(w, r, http.StatusUnsupportedMediaType, "content type not supported")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers based on OWASP recommendations
		headers := w.Header()

		// Prevent MIME-sniffing
		headers.Set("X-Content-Type-Options", "nosniff")

		// XSS protection
		headers.Set("X-XSS-Protection", "1; mode=block")

		// Frame options to prevent clickjacking
		headers.Set("X-Frame-Options", "DENY")

		// Strict Transport Security
		if app.config.env == "production" {
			headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Content Security Policy
		headers.Set("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'self'; "+
				"style-src 'self'; "+
				"img-src 'self'; "+
				"connect-src 'self'")

		// Referrer Policy
		headers.Set("Referrer-Policy", "same-origin")

		// Permissions Policy
		headers.Set("Permissions-Policy",
			"accelerometer=(), "+
				"camera=(), "+
				"geolocation=(), "+
				"gyroscope=(), "+
				"magnetometer=(), "+
				"microphone=(), "+
				"payment=(), "+
				"usb=()")

		next.ServeHTTP(w, r)
	})
}

func (app *application) tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get trace ID from header or generate new one
		traceID := r.Header.Get("X-Request-ID")
		if traceID == "" {
			traceID = middleware.GetReqID(r.Context())
		}

		// Add trace ID to response headers
		w.Header().Set("X-Request-ID", traceID)

		// Create a new context with trace information
		ctx := r.Context()
		ctx = context.WithValue(ctx, traceIDContextKey, traceID)
		ctx = context.WithValue(ctx, startTimeContextKey, time.Now())
		ctx = context.WithValue(ctx, userAgentContextKey, r.UserAgent())
		ctx = context.WithValue(ctx, remoteAddrContextKey, r.RemoteAddr)

		// Log request start
		app.logger.Info("request started",
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)

		// Create response wrapper to capture status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Process request
		next.ServeHTTP(ww, r.WithContext(ctx))

		// Calculate request duration
		duration := time.Since(ctx.Value(startTimeContextKey).(time.Time))

		// Log request completion
		app.logger.Info("request completed",
			"trace_id", traceID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", duration,
			"bytes_written", ww.BytesWritten(),
		)
	})
}
