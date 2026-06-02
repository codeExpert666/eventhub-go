-- name: CreateAuthSession :execresult
INSERT INTO auth_sessions (
    session_id,
    user_id,
    refresh_token_hash,
    status,
    issued_at,
    refresh_expires_at,
    last_refreshed_at,
    last_seen_at,
    revoked_at,
    revoke_reason,
    client_ip_hash,
    user_agent_hash,
    user_agent_summary,
    version
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindAuthSessionBySessionID :one
SELECT id, session_id, user_id, refresh_token_hash, status, issued_at, refresh_expires_at,
       last_refreshed_at, last_seen_at, revoked_at, revoke_reason, client_ip_hash,
       user_agent_hash, user_agent_summary, version, created_at, updated_at
FROM auth_sessions
WHERE session_id = ?;

-- name: FindAuthSessionByRefreshTokenHash :one
SELECT id, session_id, user_id, refresh_token_hash, status, issued_at, refresh_expires_at,
       last_refreshed_at, last_seen_at, revoked_at, revoke_reason, client_ip_hash,
       user_agent_hash, user_agent_summary, version, created_at, updated_at
FROM auth_sessions
WHERE refresh_token_hash = ?;

-- name: RotateAuthSessionRefreshToken :execresult
UPDATE auth_sessions
SET refresh_token_hash = ?,
    refresh_expires_at = ?,
    last_refreshed_at = ?,
    last_seen_at = ?,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE session_id = ?
  AND refresh_token_hash = ?
  AND version = ?
  AND status = 'ACTIVE'
  AND refresh_expires_at > ?;

-- name: UpdateAuthSessionLastSeenAt :execresult
UPDATE auth_sessions
SET last_seen_at = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE session_id = ?
  AND status = 'ACTIVE';

-- name: RevokeAuthSessionBySessionID :execresult
UPDATE auth_sessions
SET status = 'REVOKED',
    revoked_at = ?,
    revoke_reason = ?,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE session_id = ?
  AND status = 'ACTIVE';

-- name: UpdateAuthSessionStatus :execresult
UPDATE auth_sessions
SET status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE session_id = ?;
