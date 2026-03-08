# ── build stage (Go) ──────────────────────────────────────────────────────────
# ╰─➤  fly deploy --build-arg ORT_VER=1.17.0 --build-arg BUILD_TAGS=genai2

FROM golang:1.23-bullseye AS build

ARG BUILD_TAGS="" 

RUN apt-get update \
 && apt-get install -y --no-install-recommends git gcc pkg-config \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -tags="${BUILD_TAGS}" -o /out/schma ./cmd/schma

# ── runtime stage (Debian bookworm + Python venv) ─────────────────────────────
FROM debian:bookworm-slim

# System deps (no compiler toolchain needed)
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      procps curl ca-certificates \
      libstdc++6 libgcc-s1 libgomp1 \
      python3 python3-venv python3-pip \
 && rm -rf /var/lib/apt/lists/*

# Go app
COPY --from=build /out/schma /usr/local/bin/schma
RUN chmod +x /usr/local/bin/schma


# --- System ONNX Runtime for the Go binary (keep Python isolated) ---
ARG ORT_VER=1.17.0
ENV ONNX_RUNTIME_PATH=/opt/onnxruntime/lib \
    ONNX_RUNTIME_SO=/opt/onnxruntime/lib/libonnxruntime.so

RUN set -eux; \
  mkdir -p /opt/onnxruntime; \
  apt-get update; \
  apt-get install -y --no-install-recommends curl ca-certificates; \
  curl -fL -o /tmp/ort.tgz \
    "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VER}/onnxruntime-linux-x64-${ORT_VER}.tgz"; \
  tar -xzf /tmp/ort.tgz -C /opt/onnxruntime --strip-components=1; \
  rm -f /tmp/ort.tgz; \
  # Ensure we actually have the library, or fail the build loudly
  test -e /opt/onnxruntime/lib/libonnxruntime.so.*; \
  ln -sf "$(readlink -f /opt/onnxruntime/lib/libonnxruntime.so.*)" /opt/onnxruntime/lib/libonnxruntime.so; \
  ls -lh /opt/onnxruntime/lib


ENV ONNX_RUNTIME_PATH=/opt/onnxruntime/lib ONNX_RUNTIME_SO=/opt/onnxruntime/lib/libonnxruntime.so HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_HOME=/data/hf TRANSFORMERS_CACHE=/data/hf



 
# # ORT for Go ONLY — keep it out of Python
# COPY runtime/onnxruntime-linux-x64-1.17.0/lib/ /opt/onnxruntime/lib/
# ENV ONNX_RUNTIME_PATH=/opt/onnxruntime/lib

# # Make the path your Go binary expects available
# RUN ln -sf /opt/onnxruntime/lib/libonnxruntime.so.1.17.0 /usr/lib/libonnxruntime.so.1.17.0 \
#  && ln -sf /opt/onnxruntime/lib/libonnxruntime.so.1.17.0 /usr/lib/libonnxruntime.so.1




# Python venv
RUN python3 -m venv /opt/venv \
 && /opt/venv/bin/pip install --upgrade pip setuptools wheel
ENV VIRTUAL_ENV=/opt/venv
ENV PATH="/opt/venv/bin:${PATH}"

# Make pip lean & deterministic
ENV PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_ONLY_BINARY=:all: \
    PYTHONDONTWRITEBYTECODE=1

# Constraints + requirements
COPY sidecar/constraints.txt  /opt/sidecars/constraints.txt
COPY sidecar/requirements.txt /opt/sidecars/requirements.txt
ENV PIP_CONSTRAINT=/opt/sidecars/constraints.txt

# Install NumPy/ORT first; then the rest (excluding NeMo & punctuators); then punctuators w/o deps
# Install NumPy/ORT first; then the rest (excluding NeMo & punctuators); then punctuators w/o deps
RUN set -eux; \
    pip install "numpy==1.26.4" "onnxruntime==1.17.0"; \
    awk '!/^( *nemo[-_]text[-_]processing| *punctuators| *numpy| *onnxruntime)\b/' /opt/sidecars/requirements.txt > /tmp/req.nopunct.txt; \
    pip install -r /tmp/req.nopunct.txt; \
    pip install --no-deps "punctuators==0.0.7"; \
    python -c "import numpy, onnxruntime; print('numpy:', numpy.__version__); print('onnxruntime:', onnxruntime.__version__)"; \
    rm -rf /root/.cache /usr/local/share/.cache /tmp/*; \
    if [ -d "$VIRTUAL_ENV" ]; then \
      find "$VIRTUAL_ENV" -type f -name '*.py[co]' -delete; \
      find "$VIRTUAL_ENV" -type d -name '__pycache__' -prune -exec rm -rf {} +; \
      find "$VIRTUAL_ENV" -type d \( -name tests -o -name test \) -prune -exec rm -rf {} +; \
    fi


# Sidecar code
COPY sidecar_norm /opt/sidecars/sidecar_norm
COPY sidecar_tok  /opt/sidecars/sidecar_tok

# Runtime env (HF cache on volume)
ENV MODEL_DIR=/data/models
ENV NORM_UDS=/tmp/norm.sock
ENV TOK_UDS=/tmp/tok.sock
ENV HF_HOME=/data/hf TRANSFORMERS_CACHE=/data/hf

# Entrypoint
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
