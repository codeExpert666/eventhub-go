-- name: CountUsersByUsername :one
SELECT COUNT(*)
FROM users
WHERE username = ?;

-- name: CountUsersByEmail :one
SELECT COUNT(*)
FROM users
WHERE email = ?;

-- name: CreateUser :execresult
INSERT INTO users (username, email, password_hash, status)
VALUES (?, ?, ?, ?);

-- name: FindUserByUsernameOrEmail :one
SELECT id, username, email, password_hash, status, created_at, updated_at
FROM users
WHERE username = ?
   OR email = ?
LIMIT 1;

-- name: FindUserByID :one
SELECT id, username, email, password_hash, status, created_at, updated_at
FROM users
WHERE id = ?;

-- name: CountUsersByCriteria :one
SELECT COUNT(*)
FROM users
WHERE (sqlc.narg('username') IS NULL OR username LIKE CONCAT('%', sqlc.narg('username'), '%'))
  AND (sqlc.narg('email') IS NULL OR email LIKE CONCAT('%', sqlc.narg('email'), '%'))
  AND (sqlc.narg('status') IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('created_at_from') IS NULL OR created_at >= sqlc.narg('created_at_from'))
  AND (sqlc.narg('created_at_to') IS NULL OR created_at <= sqlc.narg('created_at_to'))
  AND (sqlc.narg('updated_at_from') IS NULL OR updated_at >= sqlc.narg('updated_at_from'))
  AND (sqlc.narg('updated_at_to') IS NULL OR updated_at <= sqlc.narg('updated_at_to'));

-- name: FindUsersPageByCriteria :many
SELECT id, username, email, password_hash, status, created_at, updated_at
FROM users
WHERE (sqlc.narg('username') IS NULL OR username LIKE CONCAT('%', sqlc.narg('username'), '%'))
  AND (sqlc.narg('email') IS NULL OR email LIKE CONCAT('%', sqlc.narg('email'), '%'))
  AND (sqlc.narg('status') IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('created_at_from') IS NULL OR created_at >= sqlc.narg('created_at_from'))
  AND (sqlc.narg('created_at_to') IS NULL OR created_at <= sqlc.narg('created_at_to'))
  AND (sqlc.narg('updated_at_from') IS NULL OR updated_at >= sqlc.narg('updated_at_from'))
  AND (sqlc.narg('updated_at_to') IS NULL OR updated_at <= sqlc.narg('updated_at_to'))
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?;

-- name: UpdateUserStatus :execresult
UPDATE users
SET status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
