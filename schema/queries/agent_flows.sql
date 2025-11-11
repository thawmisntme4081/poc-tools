-- name: GetAgentFlowById :one
SELECT * FROM agent_flows WHERE id = $1;
-- name: CreateAgentFlow :one
INSERT INTO agent_flows (id, name, config) VALUES ($1, $2, $3) RETURNING *;
-- name: UpdateAgentFlow :one
UPDATE agent_flows SET name = $2, config = $3 where id = $1 RETURNING *;
-- name: DeleteAgentFlow :exec
DELETE FROM agent_flows WHERE id = $1;
-- name: ListAgentFlows :many
SELECT * FROM agent_flows ORDER BY created_at DESC LIMIT $1 OFFSET $2;