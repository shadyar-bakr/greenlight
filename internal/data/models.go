package data

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Movies MovieModel
}

func NewModels(db *pgx.Conn) Models {
	return Models{
		Movies: MovieModel{DB: db},
	}
}
