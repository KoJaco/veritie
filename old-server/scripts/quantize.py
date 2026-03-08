from onnxruntime.quantization import quantize_dynamic, QuantType

quantize_dynamic(
    model_input="models/bge/model.onnx",
    model_output="models/bge/model.int8.onnx",
    weight_type=QuantType.QInt8,   # QInt8 or QUInt8
)
print("✓ dynamic INT8 model written to models/bge/model.int8.onnx")
