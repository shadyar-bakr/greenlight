package main

import (
	"expvar"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func (app *application) routes() http.Handler {
	r := chi.NewRouter()

	// Core middleware - order is important
	r.Use(app.tracing)         // Should be first to trace everything
	r.Use(app.securityHeaders) // Add security headers early
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)
	r.Use(middleware.Compress(5)) // Add compression for responses

	// Application middleware
	r.Use(app.metrics)
	r.Use(app.validateRequest)
	r.Use(app.rateLimit)
	r.Use(app.authenticate)

	// CORS middleware configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   app.config.cors.trustedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.NotFound(app.notFoundResponse)
	r.MethodNotAllowed(app.methodNotAllowedResponse)

	// Health check endpoints with rate limiting exemption
	r.Group(func(r chi.Router) {
		r.Use(middleware.NoCache)
		r.Use(middleware.Throttle(1000)) // Higher limit for health checks
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("pong"))
		})
		r.Get("/health", app.healthcheckHandler)
	})

	// API routes
	r.Route("/v1", func(r chi.Router) {

		// Public routes with specific rate limits
		r.Group(func(r chi.Router) {
			r.Use(middleware.Throttle(100)) // Lower limit for public endpoints
			r.Post("/users", app.registerUserHandler)
			r.Put("/users/activated", app.activateUserHandler)
			r.Post("/tokens/authentication", app.createAuthenticationTokenHandler)
			r.Post("/tokens/refresh", app.refreshTokenHandler)
		})

		// Protected routes - movies read
		r.Group(func(r chi.Router) {
			r.Use(app.requirePermission("movies:read"))
			r.Use(middleware.Throttle(200)) // Medium limit for read operations
			r.Get("/movies", app.listMoviesHandler)
			r.Get("/movies/{id}", app.showMovieHandler)
		})

		// Protected routes - movies write
		r.Group(func(r chi.Router) {
			r.Use(app.requirePermission("movies:write"))
			r.Use(middleware.Throttle(50)) // Lower limit for write operations
			r.Post("/movies", app.createMovieHandler)

			// Update and delete require resource-level permissions
			r.Route("/movies/{id}", func(r chi.Router) {
				r.Use(func(next http.Handler) http.Handler {
					return app.requireResourcePermission("movie", "movies:write", app.readIDParam)(next)
				})
				r.Patch("/", app.updateMovieHandler)
				r.Delete("/", app.deleteMovieHandler)
			})

			// Role management routes - admin only
			r.Group(func(r chi.Router) {
				r.Use(app.requirePermission("roles:write"))
				r.Use(middleware.Throttle(50)) // Lower limit for write operations

				// Role CRUD operations
				r.Post("/roles", app.createRoleHandler)
				r.Get("/roles", app.listRolesHandler)
				r.Get("/roles/{id}", app.showRoleHandler)
				r.Patch("/roles/{id}", app.updateRoleHandler)
				r.Delete("/roles/{id}", app.deleteRoleHandler)

				// Role assignments
				r.Post("/roles/assign", app.assignRoleHandler)
				r.Post("/roles/unassign", app.unassignRoleHandler)
				r.Get("/users/{id}/roles", app.listUserRolesHandler)
				r.Get("/roles/{id}/permissions", app.listRolePermissionsHandler)
			})
		})
	})

	// Debug routes
	if app.config.env == "development" {
		r.Group(func(r chi.Router) {
			r.Use(middleware.NoCache)
			r.Use(middleware.BasicAuth("Greenlight API", map[string]string{
				"admin": "development", // In production, use secure credentials
			}))
			r.Mount("/debug", middleware.Profiler())
			r.Get("/debug/vars", expvar.Handler().ServeHTTP)
		})
	}

	return r
}
