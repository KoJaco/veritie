# tok_server.py
from fastapi import FastAPI
from pydantic import BaseModel
from fastapi.responses import ORJSONResponse
from transformers import PreTrainedTokenizerFast
from pathlib import Path
import os, json

MODEL_DIR = Path(os.getenv("PHI_TOK_MODEL_DIR", "/data/models/phi_roberta_onnx_int8"))
# MODEL_DIR = Path(os.getenv("MODEL_DIR", "../models/phi_roberta_onnx_int8"))
TOK_FILE = MODEL_DIR / "tokenizer.json"

# build from working dir (for dev)
# TOK_FILE = Path("../models/phi_roberta_onnx_int8/tokenizer.json") # comment out for deploy

if not TOK_FILE.exists():
    raise RuntimeError(f"tokenizer.json not found at {TOK_FILE}")

# Load the fast tokenizer straight from the serialized file (no HF Hub involved)
tok = PreTrainedTokenizerFast(tokenizer_file=str(TOK_FILE))

# (Optional) ensure special tokens are set if your tokenizer.json doesn't embed them)
stm = MODEL_DIR / "special_tokens_map.json"
if stm.exists():
    try:
        with open(stm) as f:
            m = json.load(f)
        # only set if missing to avoid overriding baked-in config
        for k in ("unk_token", "bos_token", "eos_token", "sep_token", "pad_token", "cls_token", "mask_token"):
            v = m.get(k)
            if v and getattr(tok, k) is None:
                setattr(tok, k, v)
    except Exception:
        pass  # safe to ignore; tokenizer.json usually contains these

app = FastAPI(default_response_class=ORJSONResponse)

class EncodeReq(BaseModel):
    text: str
    max_length: int = 512

class BatchReq(BaseModel):
    texts: list[str]
    max_length: int = 512

@app.get("/healthz")
def healthz():
    return {"ok": True}

@app.post("/encode")
def encode(r: EncodeReq):
    enc = tok(
        r.text,
        return_offsets_mapping=True,
        truncation=True,
        max_length=r.max_length,
    )
    return {
        "input_ids": [enc["input_ids"]],
        "attention_mask": [enc["attention_mask"]],
        "offsets": [enc["offset_mapping"]],
    }

@app.post("/encode_batch")
def encode_batch(r: BatchReq):
    enc = tok(
        r.texts,
        return_offsets_mapping=True,
        truncation=True,
        max_length=r.max_length,
    )
    return {
        "input_ids": enc["input_ids"],
        "attention_mask": enc["attention_mask"],
        "offsets": enc["offset_mapping"],
    }
