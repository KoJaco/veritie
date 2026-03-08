## Deployment

After installing the fly CLI tool: https://fly.io/docs/flyctl/install/ and logging in (I need to invite you as a team member)

1. fly deploy

2.  - (optional - changing secrets)\_
    - fly secrets set GEMINI_API_KEY=copy_your_api_key
    - fly secrets set GOOGLE_STT_SERVICE_ACCOUNT_JSON="$(cat config/service_account.json)"

### Models

## distilBERT `server/models/bge`

1. mkdir -m models/bge
2. ❯ pip uninstall optimum && pip install git+https://github.com/huggingface/optimum.git
3. ❯ optimum-cli export onnx --model BAAI/bge-small-en-v1.5 models/bge
4. pip install "numpy<2" \
   onnx==1.18 \
   onnxruntime==1.17.1 \
   onnxruntime-tools==1.7.0

5. python scripts/quantize.py

After this we're referencing the model correctly for local dev (using model.int8.onnx)

## Fast Text `server/models/fasttext`

1. mkdir -m models/fasttext
2. wget https://dl.fbaipublicfiles.com/fasttext/vectors-crawl/cc.en.300.bin.gz
3. gunzip cc.en.300.bin.gz # 7 GB

## ONNX Runtime `server/runtime`

1. mkdir runtime
2. cd runtime
3. wget -q https://github.com/microsoft/onnxruntime/releases/download/v1.17.0/onnxruntime-linux-x64-1.17.0.tgz
4. tar -xzf onnxruntime-linux-x64-1.17.0.tgz onnxruntime-linux-x64-1.17.0/lib
5. rm onnxruntime-linux-x64-1.17.0.tgz
