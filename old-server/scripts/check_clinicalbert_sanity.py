from onnxruntime import InferenceSession, SessionOptions, GraphOptimizationLevel
import numpy as np, time

def load(path):
    so = SessionOptions()
    so.graph_optimization_level = GraphOptimizationLevel.ORT_ENABLE_ALL
    return InferenceSession(path, so, providers=["CPUExecutionProvider"])

fp32 = load("./models/phi_roberta_onnx/model.onnx")
int8 = load("./models/phi_roberta_onnx_int8/model_quantized.onnx")

# toy inputs: adjust to your real tokenized arrays
import json, os
from transformers import AutoTokenizer
tok = AutoTokenizer.from_pretrained("./models/phi_roberta_onnx", use_fast=True)
enc = tok("John Smith at St Mary's on 03/11/2023. Phone (02) 9123 4567.", return_tensors="np")
inputs = {"input_ids": enc["input_ids"], "attention_mask": enc["attention_mask"]}

def bench(sess, n=30):
    sess.run(None, inputs)  # warmup
    t0 = time.perf_counter()
    for _ in range(n):
        sess.run(None, inputs)
    return (time.perf_counter() - t0)/n

t_fp32 = bench(fp32)
t_int8 = bench(int8)
y_fp32 = fp32.run(None, inputs)[0].argmax(axis=-1)
y_int8 = int8.run(None, inputs)[0].argmax(axis=-1)

print("latency fp32/int8 (ms):", round(1000*t_fp32,2), round(1000*t_int8,2))
print("tokenwise equality %:", (y_fp32==y_int8).mean()*100)
