-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: AddChirps :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2

)
RETURNING *;

-- name: GetAllChirpsAsc :many
SELECT * FROM chirps ORDER BY created_at ASC;

-- name: GetAllChirpsDesc :many
SELECT * FROM chirps ORDER BY created_at DESC;

-- name: GetChirpsByAuthorAsc :many
SELECT * FROM chirps WHERE user_id = $1 ORDER BY created_at ASC;

-- name: GetChirpsByAuthorDesc :many
SELECT * FROM chirps WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetChirp :one
SELECT * FROM chirps where ID = $1;

-- name: AuthUser :one
SELECT * FROM users WHERE email = $1;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    $3,
    NULL
)
RETURNING *;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;

-- name: GetUserFromRefreshToken :one
SELECT users.* FROM users
JOIN refresh_tokens ON users.id = refresh_tokens.user_id
WHERE refresh_tokens.token = $1
AND refresh_tokens.expires_at > NOW()
AND refresh_tokens.revoked_at IS NULL;

-- name: UpdateUserDetails :exec
UPDATE users 
SET email = $1, hashed_password = $2
WHERE id = $3;

-- name: DeleteChirpByID :exec
DELETE FROM chirps 
WHERE id = $1;

-- name: UpdateMembership :exec
UPDATE  users
SET is_chirpy_red = TRUE
WHERE id = $1;