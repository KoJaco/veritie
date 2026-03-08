import onnxruntime as ort

# Load the model
model_path = "models/bge/model.onnx"
session = ort.InferenceSession(model_path)

# Print input and output names
print("Input names:", [input.name for input in session.get_inputs()])
print("Output names:", [output.name for output in session.get_outputs()]) 


# RUn this for Quantization

# python - <<'PY'
# from onnxruntime.quantization import quantize_dynamic, QuantType

# quantize_dynamic(
#     model_input  = "models/bge/model.onnx",
#     model_output = "models/bge/model.int8.onnx",
#     weight_type  = QuantType.QInt8
# )
# print("✅ Saved INT8 model to models/bge/model.int8.onnx")
# PY
