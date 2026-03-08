-- name: UpsertDraftFunctionAgg :one
INSERT INTO draft_function_aggs (
    session_id,
    app_id,
    account_id,
    function_name,
    total_detections,
    highest_score,
    avg_score,
    first_detected,
    last_detected,
    sample_args,
    version_count,
    final_call_count
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (session_id, function_name) DO UPDATE SET
    total_detections = draft_function_aggs.total_detections + EXCLUDED.total_detections,
    highest_score = GREATEST(draft_function_aggs.highest_score, EXCLUDED.highest_score),
    avg_score = (draft_function_aggs.avg_score * draft_function_aggs.total_detections + EXCLUDED.avg_score * EXCLUDED.total_detections) / (draft_function_aggs.total_detections + EXCLUDED.total_detections),
    last_detected = EXCLUDED.last_detected,
    sample_args = CASE 
        WHEN EXCLUDED.highest_score > draft_function_aggs.highest_score 
        THEN EXCLUDED.sample_args 
        ELSE draft_function_aggs.sample_args 
    END,
    version_count = draft_function_aggs.version_count + EXCLUDED.version_count,
    final_call_count = draft_function_aggs.final_call_count + EXCLUDED.final_call_count,
    updated_at = now()
RETURNING *;

-- name: GetDraftFunctionAggsBySession :many
SELECT * FROM draft_function_aggs 
WHERE session_id = $1 
ORDER BY total_detections DESC;

-- name: GetDraftFunctionAgg :one
SELECT * FROM draft_function_aggs 
WHERE session_id = $1 AND function_name = $2;

-- name: UpsertDraftFunctionStats :one
INSERT INTO draft_function_stats (
    session_id,
    app_id,
    account_id,
    total_draft_functions,
    total_final_functions,
    draft_to_final_ratio,
    unique_functions,
    avg_detection_latency,
    top_function
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (session_id) DO UPDATE SET
    total_draft_functions = EXCLUDED.total_draft_functions,
    total_final_functions = EXCLUDED.total_final_functions,
    draft_to_final_ratio = EXCLUDED.draft_to_final_ratio,
    unique_functions = EXCLUDED.unique_functions,
    avg_detection_latency = EXCLUDED.avg_detection_latency,
    top_function = EXCLUDED.top_function,
    updated_at = now()
RETURNING *;

-- name: GetDraftFunctionStats :one
SELECT * FROM draft_function_stats 
WHERE session_id = $1; 