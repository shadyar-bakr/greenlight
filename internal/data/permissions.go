package data

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Permissions []string

type PermissionsModel struct {
	Pool *pgxpool.Pool
}

func (p Permissions) Include(code string) bool {
	return slices.Contains(p, code)
}

func (m PermissionsModel) GetAllForUser(userID int64) (Permissions, error) {
	query := `
		SELECT permissions.code
		FROM permissions
		INNER JOIN users_permissions ON permissions.id = users_permissions.permission_id
		INNER JOIN users ON users.id = users_permissions.user_id
		WHERE users.id = $1
	`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission string
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (m PermissionsModel) AddForUser(userID int64, codes ...string) error {
	query := `
		INSERT INTO users_permissions (user_id, permission_id)
		SELECT $1, permissions.id
		FROM permissions
		WHERE permissions.code = ANY($2)
	`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.Pool.Exec(ctx, query, userID, codes)
	return err
}
