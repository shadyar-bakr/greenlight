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

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.Heartbeat("/ping"))
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)

	// Custom middleware
	r.Use(app.metrics)
	r.Use(app.rateLimit)
	r.Use(app.authenticate)

	// CORS middleware configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   app.config.cors.trustedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.NotFound(app.notFoundResponse)
	r.MethodNotAllowed(app.methodNotAllowedResponse)

	// API routes
	r.Route("/v1", func(r chi.Router) {
		// Public routes
		r.Get("/healthcheck", app.healthcheckHandler)
		r.Post("/users", app.registerUserHandler)
		r.Put("/users/activated", app.activateUserHandler)
		r.Post("/tokens/authentication", app.createAuthenticationTokenHandler)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(app.requirePermission("movies:read"))
			r.Get("/movies", app.listMoviesHandler)
			r.Get("/movies/{id}", app.showMovieHandler)
		})

		r.Group(func(r chi.Router) {
			r.Use(app.requirePermission("movies:write"))
			r.Post("/movies", app.createMovieHandler)
			r.Patch("/movies/{id}", app.updateMovieHandler)
			r.Delete("/movies/{id}", app.deleteMovieHandler)
		})
	})

	// Debug routes
	if app.config.env == "development" {
		r.Mount("/debug", middleware.Profiler())
		r.Get("/debug/vars", expvar.Handler().ServeHTTP)
	}

	return r
}
