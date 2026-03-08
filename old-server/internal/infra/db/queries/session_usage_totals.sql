-- name: AddSessionUsageTotal :one
INSERT INTO session_usage_totals (
    session_id,
    account_id,
    app_id,
    audio_seconds,
    prompt_tokens,
    completion_tokens,
    saved_prompt_tokens,
    cpu_active_seconds,
    cpu_idle_seconds,
    prompt_cost,
    completion_cost,
    saved_prompt_cost,
    audio_cost,
    cpu_cost,
    total_cost,
    updated_at
)
VALUES (
    @session_id,
    @account_id,
    @app_id,
    @audio_seconds,
    @prompt_tokens,
    @completion_tokens,
    @saved_prompt_tokens,
    @cpu_active_seconds,
    @cpu_idle_seconds,
    @prompt_cost,
    @completion_cost,
    @saved_prompt_cost,
    @audio_cost,
    @cpu_cost,
    @total_cost,
    now()
)
ON CONFLICT (session_id) DO UPDATE SET
    audio_seconds = session_usage_totals.audio_seconds + EXCLUDED.audio_seconds,
    prompt_tokens = session_usage_totals.prompt_tokens + EXCLUDED.prompt_tokens,
    completion_tokens = session_usage_totals.completion_tokens + EXCLUDED.completion_tokens,
    saved_prompt_tokens = session_usage_totals.saved_prompt_tokens + EXCLUDED.saved_prompt_tokens,
    cpu_active_seconds = session_usage_totals.cpu_active_seconds + EXCLUDED.cpu_active_seconds,
    cpu_idle_seconds = session_usage_totals.cpu_idle_seconds + EXCLUDED.cpu_idle_seconds,
    prompt_cost = session_usage_totals.prompt_cost + EXCLUDED.prompt_cost,
    completion_cost = session_usage_totals.completion_cost + EXCLUDED.completion_cost,
    saved_prompt_cost = session_usage_totals.saved_prompt_cost + EXCLUDED.saved_prompt_cost,
    audio_cost = session_usage_totals.audio_cost + EXCLUDED.audio_cost,
    cpu_cost = session_usage_totals.cpu_cost + EXCLUDED.cpu_cost,
    total_cost = session_usage_totals.total_cost + EXCLUDED.total_cost,
    updated_at = now()
RETURNING *;