package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/shadyar-bakr/greenlight/internal/data"
	"github.com/shadyar-bakr/greenlight/internal/validator"
)

func (app *application) createTrustedClientHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		RateLimitRPS   int    `json:"rate_limit_rps"`
		RateLimitBurst int    `json:"rate_limit_burst"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	client := &data.TrustedClient{
		Name:           input.Name,
		Description:    input.Description,
		RateLimitRPS:   input.RateLimitRPS,
		RateLimitBurst: input.RateLimitBurst,
		Enabled:        true,
	}

	v := validator.New()

	if v.Valid() {
		v.Check(input.Name != "", "name", "must be provided")
		v.Check(len(input.Name) <= 500, "name", "must not be more than 500 bytes long")
		v.Check(input.RateLimitRPS > 0, "rate_limit_rps", "must be greater than 0")
		v.Check(input.RateLimitBurst > 0, "rate_limit_burst", "must be greater than 0")
		v.Check(input.RateLimitBurst >= input.RateLimitRPS, "rate_limit_burst", "must be greater than or equal to rate_limit_rps")
	}

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.TrustedClients.Insert(client)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/trusted-clients/%d", client.ID))

	// Include the generated API key in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{
		"trusted_client": client,
		"message":        "Store the API key securely as it won't be shown again",
	}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showTrustedClientHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	clients, err := app.models.TrustedClients.GetAll()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var client *data.TrustedClient
	for _, c := range clients {
		if c.ID == id {
			client = c
			break
		}
	}

	if client == nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"trusted_client": client}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateTrustedClientHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Get the current client
	clients, err := app.models.TrustedClients.GetAll()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var client *data.TrustedClient
	for _, c := range clients {
		if c.ID == id {
			client = c
			break
		}
	}

	if client == nil {
		app.notFoundResponse(w, r)
		return
	}

	var input struct {
		Name           *string `json:"name"`
		Description    *string `json:"description"`
		RateLimitRPS   *int    `json:"rate_limit_rps"`
		RateLimitBurst *int    `json:"rate_limit_burst"`
		Enabled        *bool   `json:"enabled"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	if input.Name != nil {
		client.Name = *input.Name
	}

	if input.Description != nil {
		client.Description = *input.Description
	}

	if input.RateLimitRPS != nil {
		client.RateLimitRPS = *input.RateLimitRPS
	}

	if input.RateLimitBurst != nil {
		client.RateLimitBurst = *input.RateLimitBurst
	}

	if input.Enabled != nil {
		client.Enabled = *input.Enabled
	}

	v := validator.New()

	if v.Valid() {
		v.Check(client.Name != "", "name", "must be provided")
		v.Check(len(client.Name) <= 500, "name", "must not be more than 500 bytes long")
		v.Check(client.RateLimitRPS > 0, "rate_limit_rps", "must be greater than 0")
		v.Check(client.RateLimitBurst > 0, "rate_limit_burst", "must be greater than 0")
		v.Check(client.RateLimitBurst >= client.RateLimitRPS, "rate_limit_burst", "must be greater than or equal to rate_limit_rps")
	}

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.TrustedClients.Update(client)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"trusted_client": client}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteTrustedClientHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.TrustedClients.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "trusted client successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) listTrustedClientsHandler(w http.ResponseWriter, r *http.Request) {
	clients, err := app.models.TrustedClients.GetAll()
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"trusted_clients": clients}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) regenerateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	apiKey, err := app.models.TrustedClients.RegenerateAPIKey(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{
		"message": "API key successfully regenerated",
		"api_key": apiKey,
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
