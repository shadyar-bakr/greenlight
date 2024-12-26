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
	r.Use(middleware.Timeout(60 * time.Second))

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

	// healthcheck
	r.Get("/v1/healthcheck", app.healthcheckHandler)

	// movies routes with permissions
	r.Group(func(r chi.Router) {
		r.Use(app.requirePermission("movies:read"))
		r.Get("/v1/movies/{id}", app.showMovieHandler)
		r.Get("/v1/movies", app.listMoviesHandler)
	})

	r.Group(func(r chi.Router) {
		r.Use(app.requirePermission("movies:write"))
		r.Post("/v1/movies", app.createMovieHandler)
		r.Patch("/v1/movies/{id}", app.updateMovieHandler)
		r.Delete("/v1/movies/{id}", app.deleteMovieHandler)
	})

	// users
	r.Post("/v1/users", app.registerUserHandler)
	r.Put("/v1/users/activated", app.activateUserHandler)

	// tokens
	r.Post("/v1/tokens/authentication", app.createAuthenticationTokenHandler)

	// metrics
	r.Handle("/debug/vars", expvar.Handler())

	return r
}
