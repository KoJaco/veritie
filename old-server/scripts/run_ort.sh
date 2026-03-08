#!/bin/bash
export LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH
go run scripts/test_ort.go -model models/bge/model.int8.onnx -sentence "Create a new project called alpha"