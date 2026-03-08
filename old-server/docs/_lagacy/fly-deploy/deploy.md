# Runbook: Deploying `schma-api` on Fly.io (ONNX + sidecars)

## 0) One-time setup

```bash
fly auth login
# If you don’t already have a volume:
fly volumes create data --size 10 --region syd -a schma-api
```

## 1) Build & deploy

Your Dockerfile installs:

-   the Go binary,
-   a Python venv for sidecars,
-   **ONNX Runtime 1.17.0** to `/opt/onnxruntime/lib`,
-   sidecars and entrypoint.

Deploy:

```bash
fly deploy -a schma-api
```

## 2) Make sure the VM has enough memory

We saw OOM at \~1 GB. Bump to 2 GB **before** the heavy models load:

```bash
# Machines
fly machine list -a schma-api
fly machine update <MACHINE_ID> --memory 2048 -a schma-api
# (Alternative on Apps V1: fly scale memory 2048 -a schma-api)
```

## 3) Put models onto the mounted volume (`/data/models`)

Best path we used: **SFTP shell** and push local files to the machine.

Open an SFTP shell (pick the running machine):

```bash
fly sftp shell -a schma-api --machine <MACHINE_ID>
```

Inside the SFTP prompt (note: no `lcd` here; give local paths explicitly):

```text
# Remote working dir (optional)
cd /data/models/phi_roberta_onnx_int8

# Upload to .tmp names first (atomic-ish swap later)
put ./models/phi_roberta_onnx_int8/model_quantized.onnx /data/models/phi_roberta_onnx_int8/model_quantized.onnx.tmp
put ./models/bge/model.int8.onnx                    /data/models/bge/model.int8.onnx.tmp
put ./models/fasttext/cc.en.300.100k.vec            /data/models/fasttext/cc.en.300.100k.vec.tmp
```

## 4) (FastText only) Add header if missing

FastText text vectors need the `N D` header line. We added it in the VM:

```bash
fly ssh console -a schma-api
set -euo pipefail
f=/data/models/fasttext/cc.en.300.100k.vec.tmp
if ! head -n1 "$f" | grep -Eq '^[0-9]+\s+[0-9]+\s*$'; then
  words=$(wc -l < "$f")
  dim=$(sed -n '2p' "$f" | awk '{print NF-1}')
  { echo "$words $dim"; cat "$f"; } > "${f%.tmp}.new"
  mv -f "${f%.tmp}.new" /data/models/fasttext/cc.en.300.100k.vec
else
  mv -f "$f" /data/models/fasttext/cc.en.300.100k.vec
fi
chmod 0644 /data/models/fasttext/cc.en.300.100k.vec
```

## 5) Verify integrity (size + checksum)

On the VM:

```bash
sha256sum /data/models/phi_roberta_onnx_int8/model_quantized.onnx.tmp
sha256sum /data/models/bge/model.int8.onnx.tmp
sha256sum /data/models/fasttext/cc.en.300.100k.vec   # (after header handling)
```

On your local machine:

```bash
sha256sum ./models/phi_roberta_onnx_int8/model_quantized.onnx
sha256sum ./models/bge/model.int8.onnx
sha256sum ./models/fasttext/cc.en.300.100k.vec
```

Confirm they match. (We found the **phi** model was corrupted earlier—the checksum mismatch explained the “protobuf parsing failed” error.)

## 6) Swap `.tmp` files into place & clean up

```bash
# BGE
mv -f /data/models/bge/model.int8.onnx.tmp \
      /data/models/bge/model.int8.onnx
chmod 0644 /data/models/bge/model.int8.onnx

# PHI (RoBERTa ONNX)
mv -f /data/models/phi_roberta_onnx_int8/model_quantized.onnx.tmp \
      /data/models/phi_roberta_onnx_int8/model_quantized.onnx
chmod 0644 /data/models/phi_roberta_onnx_int8/model_quantized.onnx

# Remove any leftover .tmp files
find /data/models -type f -name '*.tmp' -delete
```

## 7) Runtime environment you kept (works)

-   **ONNX Runtime (Go only):**

    -   Installed at `/opt/onnxruntime/lib`
    -   `ONNX_RUNTIME_PATH=/opt/onnxruntime/lib`
    -   In the entrypoint, _only_ the Go process gets:

        ```bash
        export LD_LIBRARY_PATH="${ONNX_RUNTIME_PATH}:${LD_LIBRARY_PATH:-}"
        exec /usr/local/bin/schma
        ```

-   **HF offline / cache to volume** (keeps image small, avoids network):

    -   `HF_HUB_OFFLINE=1`, `TRANSFORMERS_OFFLINE=1`
    -   `HF_HOME=/data/hf`, `TRANSFORMERS_CACHE=/data/hf`

-   **Tokenizer sidecar** reads **local** artifacts:

    -   `TOK_MODEL_DIR=/data/models/phi_roberta_onnx_int8`
    -   In code: `AutoTokenizer.from_pretrained(TOK_MODEL_DIR, local_files_only=True)`

## 8) Restart and watch logs

```bash
fly machine restart <MACHINE_ID> -a schma-api
fly logs -a schma-api
```

Healthy signs we saw:

-   ONNX runtime found: `Found runtime .so (/opt/onnxruntime/lib/libonnxruntime.so.1.17.0)`
-   BGE session created
-   Normalizer initialized message
-   (After fixing PHI model) no more “protobuf parsing failed”

## 9) Quick health checks inside the VM

```bash
fly ssh console -a schma-api
# sidecars use UDS; check they’re up:
curl --unix-socket /tmp/norm.sock http://unix/healthz
curl --unix-socket /tmp/tok.sock  http://unix/healthz
```

---

## Troubleshooting crib notes

-   **`protobuf parsing failed` on PHI model**

    -   Almost always a **corrupted `.onnx`**. Re-upload via SFTP, verify `sha256sum`, then `mv` into place.

-   **`Repo id must be in the form 'repo_name' …` from `huggingface_hub`**

    -   Caused by treating a filesystem path like a hub repo.
    -   Fix: `AutoTokenizer.from_pretrained("/data/models/phi_roberta_onnx_int8", local_files_only=True)`
        (or load by file: `PreTrainedTokenizerFast(tokenizer_file="/data/models/.../tokenizer.json")`).

-   **ONNX shared library not found**

    -   Ensure `/opt/onnxruntime/lib/libonnxruntime.so.*` exists on the VM:

        ```bash
        ls -lh /opt/onnxruntime/lib
        ```

    -   Make sure your entrypoint exports `LD_LIBRARY_PATH` **only** for the Go process.

-   **OOM during startup**

    -   Increase memory to 2 GB (see step 2).
    -   Keep Uvicorn workers at 1 for the sidecars.
    -   Optionally set `MEMO_NEIGHBOURS=0` to skip fastText synonym warmup.

-   **FastText hangs / misreads**

    -   Ensure header line is present (`N D`, e.g. `99999 300`).
    -   Confirm `sha256sum` matches local file.

---

## TL;DR checklist

1. `fly deploy`
2. `fly machine update <id> --memory 2048`
3. `fly sftp shell` → `put` models to `/data/models/.../*.tmp`
4. On VM: verify `sha256sum` vs local; add FastText header if needed.
5. `mv *.tmp` into place; delete leftovers.
6. Restart machine; tail `fly logs`.
7. Sanity: curl UDS health endpoints.
