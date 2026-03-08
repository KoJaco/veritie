-- name: AddTranscript :one
INSERT INTO transcripts (
    session_id,
    text,
    is_final,
    confidence,
    stability,
    chunk_dur_sec,
    channel,
    words,
    turns,
    phrases,
    created_at
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ListTranscriptsBySession :many
SELECT * FROM transcripts WHERE session_id = $1 ORDER BY created_at ASC;

-- name: ListFinalTranscriptsBySession :many
SELECT * FROM transcripts 
WHERE session_id = $1 AND is_final = true 
ORDER BY created_at ASC;

-- name: ListTranscriptsBySessionAndConfidence :many
SELECT * FROM transcripts 
WHERE session_id = $1 AND confidence >= $2 
ORDER BY created_at ASC;
