package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Role struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	ParentID    *int64    `json:"parent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	Version     int32     `json:"version"`
}

type RoleModel struct {
	Pool *pgxpool.Pool
}

// Insert adds a new role to the database
func (m RoleModel) Insert(role *Role) error {
	query := `
		INSERT INTO roles (name, description, parent_id)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, version`

	args := []any{role.Name, role.Description, role.ParentID}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.Pool.QueryRow(ctx, query, args...).Scan(&role.ID, &role.CreatedAt, &role.Version)
}

// Get retrieves a specific role from the database
func (m RoleModel) Get(id int64) (*Role, error) {
	query := `
		SELECT id, name, description, parent_id, created_at, version
		FROM roles
		WHERE id = $1`

	var role Role

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.Pool.QueryRow(ctx, query, id).Scan(
		&role.ID,
		&role.Name,
		&role.Description,
		&role.ParentID,
		&role.CreatedAt,
		&role.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &role, nil
}

// Update updates a specific role in the database
func (m RoleModel) Update(role *Role) error {
	query := `
		UPDATE roles
		SET name = $1, description = $2, parent_id = $3, version = version + 1
		WHERE id = $4 AND version = $5
		RETURNING version`

	args := []any{
		role.Name,
		role.Description,
		role.ParentID,
		role.ID,
		role.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.Pool.QueryRow(ctx, query, args...).Scan(&role.Version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

// Delete removes a role from the database
func (m RoleModel) Delete(id int64) error {
	query := `
		DELETE FROM roles
		WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.Pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetAll retrieves all roles from the database
func (m RoleModel) GetAll() ([]*Role, error) {
	query := `
		SELECT id, name, description, parent_id, created_at, version
		FROM roles
		ORDER BY id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*Role

	for rows.Next() {
		var role Role

		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.ParentID,
			&role.CreatedAt,
			&role.Version,
		)
		if err != nil {
			return nil, err
		}

		roles = append(roles, &role)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

// GetAllForUser retrieves all roles assigned to a specific user
func (m RoleModel) GetAllForUser(userID int64) ([]*Role, error) {
	query := `
		SELECT r.id, r.name, r.description, r.parent_id, r.created_at, r.version
		FROM roles r
		INNER JOIN users_roles ur ON ur.role_id = r.id
		WHERE ur.user_id = $1
		ORDER BY r.id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*Role

	for rows.Next() {
		var role Role

		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.ParentID,
			&role.CreatedAt,
			&role.Version,
		)
		if err != nil {
			return nil, err
		}

		roles = append(roles, &role)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

// AssignToUser assigns a role to a user
func (m RoleModel) AssignToUser(userID, roleID, grantedBy int64) error {
	query := `
		INSERT INTO users_roles (user_id, role_id, granted_by)
		VALUES ($1, $2, $3)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.Pool.Exec(ctx, query, userID, roleID, grantedBy)
	return err
}

// UnassignFromUser removes a role from a user
func (m RoleModel) UnassignFromUser(userID, roleID int64) error {
	query := `
		DELETE FROM users_roles
		WHERE user_id = $1 AND role_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.Pool.Exec(ctx, query, userID, roleID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetAllPermissions retrieves all permissions for a role, including inherited ones
func (m RoleModel) GetAllPermissions(roleID int64) (Permissions, error) {
	query := `
		WITH RECURSIVE role_hierarchy AS (
			-- Base case: start with the given role
			SELECT id, parent_id
			FROM roles
			WHERE id = $1
			
			UNION
			
			-- Recursive case: get all parent roles
			SELECT r.id, r.parent_id
			FROM roles r
			INNER JOIN role_hierarchy rh ON r.id = rh.parent_id
		)
		SELECT DISTINCT p.code
		FROM permissions p
		INNER JOIN roles_permissions rp ON p.id = rp.permission_id
		INNER JOIN role_hierarchy rh ON rp.role_id = rh.id
		ORDER BY p.code`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.Pool.Query(ctx, query, roleID)
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

// AssignPermission assigns a permission to a role
func (m RoleModel) AssignPermission(roleID, permissionID int64) error {
	query := `
		INSERT INTO roles_permissions (role_id, permission_id)
		VALUES ($1, $2)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.Pool.Exec(ctx, query, roleID, permissionID)
	return err
}

// UnassignPermission removes a permission from a role
func (m RoleModel) UnassignPermission(roleID, permissionID int64) error {
	query := `
		DELETE FROM roles_permissions
		WHERE role_id = $1 AND permission_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.Pool.Exec(ctx, query, roleID, permissionID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}
