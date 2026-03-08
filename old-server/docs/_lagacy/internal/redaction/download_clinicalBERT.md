## 🔽 Step-by-Step: Download + Convert ClinicalBERT → ONNX

### 1️⃣ Install the required tools

```bash
pip install --upgrade transformers
pip install --upgrade optimum[onnxruntime]  # includes ONNX export tooling
```

> `optimum-cli` will now be available in your PATH.

---

### 2️⃣ Create a folder for the model (optional)

```bash
mkdir -p ./models/phi_clinicalbert
cd ./models/phi_clinicalbert
```

---

### 3️⃣ Download the HuggingFace model locally

You can do this with:

```bash
from transformers import AutoModelForTokenClassification, AutoTokenizer

model = AutoModelForTokenClassification.from_pretrained("emilyalsentzer/Bio_ClinicalBERT")
tokenizer = AutoTokenizer.from_pretrained("emilyalsentzer/Bio_ClinicalBERT")

model.save_pretrained("./")
tokenizer.save_pretrained("./")
```

> Save this as `download_bioclinicalbert.py` and run:

```bash
python download_bioclinicalbert.py
```

---

### 4️⃣ Export to ONNX

```bash
optimum-cli export onnx \
  --model obi/deid_roberta_i2b2 \
  --task token-classification \
  --output ./models/phi_roberta_onnx
```

This will produce:

```
./models/phi_roberta_onnx/
  ├─ model.onnx
  └─ tokenizer.json
  etc
```

-- assure that inputs: input_ids, attention_mask -> [batch_size, sequence_length]
-- outputs: logits -> [batch_size, sequence_length, num_labels]
-- num_labels = 45 (matches PHI tag set for checkpoint) --> check config.json -> id2label

> ✅ You now have an ONNX‐ready `ClinicalBERT` token classification model for phi

### Quantize the model

```bash
optimum-cli onnxruntime quantize --onnx_model ./models/phi_roberta_onnx --output ./models/phi_roberta_onnx_int8 --avx2
```

### Check size diff

```bash
du -h ./models/phi_roberta_onnx/model.onnx # 1.4G
du -h ./models/phi_roberta_onnx_int8/model_quantized.onnx # 340M
```

### Sanity Check

```bash
python ./scripts/check_clinicalbert_sanity.py

# latency fp32/int8 (ms): 69.33 27.52
# tokenwise equality %: 100.0

```

---

### 5️⃣ Load in your Go runtime

Change your `phi_redactor.New()` to point to:

```

models/phi_clinicalbert/onnx/model.onnx

```

---

### 🟢 Complete Bash Summary (if you prefer raw shell)

```bash
# Install tools
pip install --upgrade transformers
pip install --upgrade optimum[onnxruntime]

# Download model
python - << 'EOF'
from transformers import AutoModelForTokenClassification, AutoTokenizer

model = AutoModelForTokenClassification.from_pretrained("emilyalsentzer/Bio_ClinicalBERT")
tokenizer = AutoTokenizer.from_pretrained("emilyalsentzer/Bio_ClinicalBERT")
model.save_pretrained("./models/phi_clinicalbert")
tokenizer.save_pretrained("./models/phi_clinicalbert")
EOF

# Export to ONNX
optimum-cli export onnx \
  --model ./models/phi_clinicalbert \
  --task token-classification \
  --output ./models/phi_clinicalbert/onnx
```
