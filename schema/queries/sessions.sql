-- name: CreateSession :one
INSERT INTO sessions (id, created_by, agent_flow_id, title) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: UpdateSessionTurnCount :exec
UPDATE sessions SET turn_count = $2 WHERE id = $1;

-- name: SessionAddChatHistory :one
INSERT INTO session_history (id, session_id, content, stop_reason, node) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: GetSessionHistoryBySessionID :many
SELECT * FROM session_history WHERE session_id = $1 ORDER BY created_at ASC;

-- name: GetSessionsByUserID :many
SELECT * FROM sessions WHERE created_by = $1 ORDER BY created_at ASC;

-- name: UpdateSessionName :exec
UPDATE sessions SET title = $2 WHERE id = $1;

-- name: DeleteSessionByID :exec
DELETE FROM sessions WHERE id = $1;