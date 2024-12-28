package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Movies              MovieModel
	Permissions         PermissionsModel
	Tokens              TokenModel
	Users               UserModel
	ResourcePermissions ResourcePermissionModel
	Roles               RoleModel
	TrustedClients      TrustedClientModel
}

func NewModels(pool *pgxpool.Pool) Models {
	return Models{
		Movies:              MovieModel{Pool: pool},
		Permissions:         PermissionsModel{Pool: pool},
		Tokens:              TokenModel{Pool: pool},
		Users:               UserModel{Pool: pool},
		ResourcePermissions: ResourcePermissionModel{Pool: pool},
		Roles:               RoleModel{Pool: pool},
		TrustedClients:      TrustedClientModel{Pool: pool},
	}
}
