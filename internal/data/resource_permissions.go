package data

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResourcePermission represents a permission for a specific resource
type ResourcePermission struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   int64     `json:"resource_id"`
	Permission   string    `json:"permission"`
	GrantedBy    *int64    `json:"granted_by,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// ResourcePermissionModel wraps a database connection pool
type ResourcePermissionModel struct {
	Pool *pgxpool.Pool
}

// Grant adds a new resource-level permission for a user
func (m ResourcePermissionModel) Grant(rp *ResourcePermission) error {
	query := `
		INSERT INTO resource_permissions (user_id, resource_type, resource_id, permission, granted_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	args := []any{
		rp.UserID,
		rp.ResourceType,
		rp.ResourceID,
		rp.Permission,
		rp.GrantedBy,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.Pool.QueryRow(ctx, query, args...).Scan(&rp.ID, &rp.CreatedAt)
}

// Revoke removes a resource-level permission from a user
func (m ResourcePermissionModel) Revoke(userID int64, resourceType string, resourceID int64, permission string) error {
	query := `
		DELETE FROM resource_permissions
		WHERE user_id = $1 AND resource_type = $2 AND resource_id = $3 AND permission = $4`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.Pool.Exec(ctx, query, userID, resourceType, resourceID, permission)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// HasPermission checks if a user has a specific permission for a resource
func (m ResourcePermissionModel) HasPermission(userID int64, resourceType string, resourceID int64, permission string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM resource_permissions
			WHERE user_id = $1 AND resource_type = $2 AND resource_id = $3 AND permission = $4
		)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var exists bool
	err := m.Pool.QueryRow(ctx, query, userID, resourceType, resourceID, permission).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// GetResourcePermissions gets all permissions for a specific resource
func (m ResourcePermissionModel) GetResourcePermissions(resourceType string, resourceID int64) ([]*ResourcePermission, error) {
	query := `
		SELECT id, user_id, resource_type, resource_id, permission, granted_by, created_at
		FROM resource_permissions
		WHERE resource_type = $1 AND resource_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query, resourceType, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []*ResourcePermission

	for rows.Next() {
		var permission ResourcePermission

		err := rows.Scan(
			&permission.ID,
			&permission.UserID,
			&permission.ResourceType,
			&permission.ResourceID,
			&permission.Permission,
			&permission.GrantedBy,
			&permission.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, &permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

// GetUserResourcePermissions gets all resource permissions for a user
func (m ResourcePermissionModel) GetUserResourcePermissions(userID int64, resourceType string) ([]*ResourcePermission, error) {
	query := `
		SELECT id, user_id, resource_type, resource_id, permission, granted_by, created_at
		FROM resource_permissions
		WHERE user_id = $1 AND resource_type = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query, userID, resourceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions []*ResourcePermission

	for rows.Next() {
		var permission ResourcePermission

		err := rows.Scan(
			&permission.ID,
			&permission.UserID,
			&permission.ResourceType,
			&permission.ResourceID,
			&permission.Permission,
			&permission.GrantedBy,
			&permission.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, &permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}
