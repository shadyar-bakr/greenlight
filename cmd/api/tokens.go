package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/shadyar-bakr/greenlight/internal/data"
	"github.com/shadyar-bakr/greenlight/internal/validator"
)

func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the email and password from the request body.
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Validate the email and password provided by the client.
	v := validator.New()

	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Lookup the user record based on the email address. If no matching user was
	// found, then we call the app.invalidCredentialsResponse() helper to send a 401
	// Unauthorized response to the client (we will create this helper in a moment).
	user, err := app.models.Users.GetByEmail(r.Context(), input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Check if the provided password matches the actual password for the user.
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// If the passwords don't match, then we call the app.invalidCredentialsResponse()
	// helper again and return.
	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	// Generate both access and refresh tokens
	accessToken, refreshToken, err := app.models.Tokens.NewPair(user.ID, 15*time.Minute, 24*time.Hour)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return both tokens in the response
	err = app.writeJSON(w, http.StatusCreated, envelope{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateTokenPlaintext(v, input.RefreshToken)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Verify and get the refresh token
	refreshToken, err := app.models.Tokens.GetRefreshToken(input.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrInvalidToken):
			app.invalidAuthenticationTokenResponse(w, r)
		case errors.Is(err, data.ErrExpiredToken):
			app.expiredAuthenticationTokenResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Generate a new token pair
	accessToken, newRefreshToken, err := app.models.Tokens.NewPair(refreshToken.UserID, 15*time.Minute, 24*time.Hour)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Delete the old refresh token
	err = app.models.Tokens.DeleteAllForUser(data.ScopeRefresh, refreshToken.UserID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Return the new token pair
	err = app.writeJSON(w, http.StatusOK, envelope{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
