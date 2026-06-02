-- name: FindRoleByCode :one
SELECT id, code, name, description, created_at
FROM roles
WHERE code = ?;

-- name: FindRoleCodesByUserID :many
SELECT r.code
FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = ?
ORDER BY r.code ASC;

-- name: FindRoleCodesByUserIDs :many
SELECT ur.user_id,
       r.code AS role_code
FROM user_roles ur
JOIN roles r ON r.id = ur.role_id
WHERE ur.user_id IN (sqlc.slice('user_ids'))
ORDER BY ur.user_id ASC, r.code ASC;

-- name: AddRoleToUser :execresult
INSERT INTO user_roles (user_id, role_id)
VALUES (?, ?);
