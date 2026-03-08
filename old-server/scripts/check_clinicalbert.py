from onnxruntime import InferenceSession
from transformers import AutoTokenizer
import numpy as np, json, os

MODEL_DIR = "./models/phi_roberta_onnx"

# 1) Load label map
cfg = json.load(open(os.path.join(MODEL_DIR, "config.json")))
id2label = {int(k): v for k, v in cfg["id2label"].items()}

# 2) Tokenize a sample
tok = AutoTokenizer.from_pretrained(MODEL_DIR, use_fast=True)
text = "Patient John Smith visited St. Mary's Hospital on 03/11/2023. Phone: (02) 9123 4567."
enc = tok(text, return_tensors="np", return_offsets_mapping=True, truncation=True)
inputs = {
    "input_ids": enc["input_ids"],
    "attention_mask": enc["attention_mask"],
}

# 3) Run ORT
sess = InferenceSession(os.path.join(MODEL_DIR, "model.onnx"), providers=["CPUExecutionProvider"])
logits = sess.run(None, inputs)[0]                 # [1, seq, num_labels]
pred_ids = logits.argmax(axis=-1)[0]               # [seq]

# 4) Decode spans (BIO/BILOU-ish)
offsets = enc["offset_mapping"][0]
tokens  = tok.convert_ids_to_tokens(enc["input_ids"][0])

spans = []
cur = None
for i, pid in enumerate(pred_ids):
    lab = id2label[int(pid)]
    if lab.startswith(("B-","U-")):
        # close previous
        if cur: spans.append(cur); cur = None
        typ = lab[2:]
        start,end = offsets[i]
        if lab.startswith("U-"):
            spans.append({"label": typ, "start": int(start), "end": int(end)})
        else:
            cur = {"label": typ, "start": int(start), "end": int(end)}
    elif lab.startswith(("I-","L-")) and cur:
        cur["end"] = int(offsets[i][1])
        if lab.startswith("L-"):
            spans.append(cur); cur = None
    else:
        if cur: spans.append(cur); cur = None
if cur: spans.append(cur)

print("Predicted spans:", spans)

def merge_spans(spans):
    # assumes spans sorted by start
    spans = sorted(spans, key=lambda s: s["start"])
    merged = []
    for s in spans:
        if not merged:
            merged.append(s)
        else:
            last = merged[-1]
            if s["label"] == last["label"] and s["start"] <= last["end"]:
                # contiguous/overlapping with same label → merge
                last["end"] = max(last["end"], s["end"])
            else:
                merged.append(s)
    return merged

spans = merge_spans(spans)

# 5) Mask text
masked_chars = []
for idx, ch in enumerate(text):
    # find any span covering this char
    match = next((sp for sp in spans if sp["start"] <= idx < sp["end"]), None)
    if match:
        if not masked_chars or not masked_chars[-1].startswith("["):  # insert tag once
            masked_chars.append(f"[{match['label']}]")
        # skip actual char
    else:
        masked_chars.append(ch)

masked_text = "".join(masked_chars)
print("Masked:", masked_text)
