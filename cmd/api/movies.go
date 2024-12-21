package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shadyar-bakr/greenlight/internal/data"
	"github.com/shadyar-bakr/greenlight/internal/validator"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		app.methodNotAllowedResponse(w, r)
		return
	}

	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}

	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	err = app.models.Movies.Insert(ctx, movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

	err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		app.methodNotAllowedResponse(w, r)
		return
	}

	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("invalid id parameter: %s", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	movie, err := app.models.Movies.Get(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		app.methodNotAllowedResponse(w, r)
		return
	}

	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("invalid id parameter: %s", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	movie, err := app.models.Movies.Get(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	if r.Header.Get("X-Expected-Version") != "" {
		if strconv.Itoa(int(movie.Version)) != r.Header.Get("X-Expected-Version") {
			app.editConflictResponse(w, r)
			return
		}
	}

	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Movies.Update(ctx, movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		app.methodNotAllowedResponse(w, r)
		return
	}

	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, fmt.Errorf("invalid id parameter: %s", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	err = app.models.Movies.Delete(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie deleted successfully"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	// Accept only GET requests
	if r.Method != http.MethodGet {
		app.methodNotAllowedResponse(w, r)
		return
	}

	var input struct {
		Title  string
		Genres []string
		data.Filters
	}

	// Get the url.Values map from the query string
	v := validator.New()
	qs := r.URL.Query()

	// Extract title search parameter
	input.Title = app.readString(qs, "title", "")

	// Extract genres search parameter
	input.Genres = app.readCSV(qs, "genres", []string{})

	// Extract pagination parameters with defaults
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)

	// Extract sort parameter with default
	input.Filters.SortBy = app.readString(qs, "sort", "id")
	input.Filters.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	// Validate filters
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	// Get the movies from the database
	movies, metadata, err := app.models.Movies.GetAll(ctx, input.Title, input.Genres, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Send a JSON response containing the movie data
	err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
