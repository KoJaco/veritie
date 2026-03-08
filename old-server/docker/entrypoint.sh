#!/usr/bin/env bash
set -euo pipefail

# Export this at end
LD_SAVED="${LD_LIBRARY_PATH:-}"
unset LD_LIBRARY_PATH

# ---------- Config / defaults ----------
NORM_UDS="${NORM_UDS:-/tmp/norm.sock}"
TOK_UDS="${TOK_UDS:-/tmp/tok.sock}"
MODEL_DIR="${MODEL_DIR:-/data/models}"
export TOK_MODEL_DIR="${TOK_MODEL_DIR:-${MODEL_DIR}/phi_roberta_onnx_int8}"

# Ensure HF caches go to vol
export HF_HOME="${HF_HOME:-/data/hf}"
export TRANSFORMERS_CACHE="${HF_HOME}"

# system ORT for Go only (Python must NOT see this)
ONNX_RUNTIME_PATH="${ONNX_RUNTIME_PATH:-/opt/onnxruntime/lib}"

UVICORN_LOG_LEVEL="${UVICORN_LOG_LEVEL:-info}"
NORM_WORKERS="${NORM_WORKERS:-1}"
TOK_WORKERS="${TOK_WORKERS:-1}"

# ---------- Prep filesystem ----------
mkdir -p /tmp "${MODEL_DIR}"
chmod 1777 /tmp || true
rm -f "${NORM_UDS}" "${TOK_UDS}"
umask 000

# ---------- Start sidecars (DO NOT leak LD_LIBRARY_PATH to Python) ----------
( unset LD_LIBRARY_PATH
  exec python3 -m uvicorn sidecar_norm.norm_server:app \
    --app-dir /opt/sidecars \
    --uds "${NORM_UDS}" \
    --workers "${NORM_WORKERS}" \
    --log-level "${UVICORN_LOG_LEVEL}"
) &  NORM_PID=$!

( unset LD_LIBRARY_PATH
  TOK_MODEL_DIR="${TOK_MODEL_DIR}" exec python3 -m uvicorn sidecar_tok.tok_server:app \
    --app-dir /opt/sidecars \
    --uds "${TOK_UDS}" \
    --workers "${TOK_WORKERS}" \
    --log-level "${UVICORN_LOG_LEVEL}"
) &  TOK_PID=$!

# ---------- Wait for sockets ----------
for _ in $(seq 1 120); do [[ -S "${NORM_UDS}" ]] && break; sleep 0.5; done
[[ -S "${NORM_UDS}" ]] || { echo "ERROR: ${NORM_UDS} not created"; ps aux | sed -n '1,200p' || true; exit 1; }

for _ in $(seq 1 120); do [[ -S "${TOK_UDS}" ]] && break; sleep 0.5; done
[[ -S "${TOK_UDS}" ]] || { echo "ERROR: ${TOK_UDS} not created"; ps aux | sed -n '1,200p' || true; exit 1; }

chmod 666 "${NORM_UDS}" "${TOK_UDS}" || true

# Sanity: warn if models aren’t present
for d in "${MODEL_DIR}/bge" "${MODEL_DIR}/fasttext" "${MODEL_DIR}/phi_roberta_onnx_int8"; do
  [[ -d "$d" ]] || echo "WARNING: expected model dir missing: $d"
done

echo "Sidecars ready (UDS: ${NORM_UDS}, ${TOK_UDS}). Launching schma..."

# ---------- Run Go app with system ORT available ----------
export LD_LIBRARY_PATH="${ONNX_RUNTIME_PATH}:${LD_LIBRARY_PATH:-}"
exec /usr/local/bin/schma
