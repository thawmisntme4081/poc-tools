-- name: CreateUser :one
INSERT INTO users (name, email, provider)
VALUES ($1, $2, $3)
RETURNING id, name, email, provider, created_at, updated_at;

-- name: GetUsers :many
SELECT * FROM users;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUser :exec
UPDATE users SET name = $2, email = $3, provider = $4 WHERE id = $1 RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;