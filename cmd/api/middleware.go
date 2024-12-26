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
	return func(next http.Handler) http.Handler {
		fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := app.contextGetUser(r)

			permissions, err := app.models.Permissions.GetAllForUser(user.ID)
			if err != nil {
				app.serverErrorResponse(w, r, err)
				return
			}

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
