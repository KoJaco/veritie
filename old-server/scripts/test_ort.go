// scripts/test_ort.go
//
// Latency sanity‑check for an INT8 ONNX embedding model using:
//
//   - github.com/sugarme/tokenizer/pretrained  (load tokenizer.json)
//   - github.com/sugarme/tokenizer            (core types)
//   - github.com/yalue/onnxruntime_go         (pure‑Go ORT bindings)
//
// Build / run:
//
//	go run scripts/test_ort.go \
//	     -model models/bge/model.onnx \
//	     -sentence "create a new project alpha"
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
	"schma.ai/internal/pkg/paths"

	tok "github.com/sugarme/tokenizer"
	tokpre "github.com/sugarme/tokenizer/pretrained"
)

var (
	modelPath = flag.String("model", "models/bge/model.int8.onnx", "path to ONNX model")
	sentence  = flag.String("sentence", "hello world", "sentence to embed")
)

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func toInt64(ids []int) []int64 {
	o := make([]int64, len(ids))
	for i, v := range ids {
		o[i] = int64(v)
	}
	return o
}

func main() {
	flag.Parse()

	/* ------------------------------------------------------------------ */
	/* 1. Load tokenizer ------------------------------------------------- */
	base := paths.ModelDir()
	bgeTokenizerPath := filepath.Join(base, "bge", "tokenizer.json")

	tokenizer, err := tokpre.FromFile(bgeTokenizerPath)
	must(err)

	enc, err := tokenizer.Encode(tok.NewSingleEncodeInput(tok.NewInputSequence(*sentence)), false)
	must(err)

	inputIDs := toInt64(enc.Ids)
	attnMask := make([]int64, len(inputIDs))
	for i := range attnMask {
		attnMask[i] = 1 // simple full mask
	}

	seqLen := int64(len(inputIDs))
	shape := onnx.NewShape(1, seqLen)

	/* ------------------------------------------------------------------ */
	/* 2.  Init ONNX runtime ------------------------------------------- */
	onnx.SetSharedLibraryPath("server/runtime/libonnxruntime.so") // no‑op if already on path
	must(onnx.InitializeEnvironment())
	defer onnx.DestroyEnvironment()

	/* ------------------------------------------------------------------ */
	/* 3.  Build tensors ------------------------------------------------ */
	idTensor, err := onnx.NewTensor[int64](shape, inputIDs)
	must(err)
	defer idTensor.Destroy()

	maskTensor, err := onnx.NewTensor[int64](shape, attnMask)
	must(err)
	defer maskTensor.Destroy()

	/* ------------------------------------------------------------------ */
	/* 4.  Run inference ------------------------------------------------ */

	outShape := onnx.NewShape(1, 384) // BGE‑small → 384‑D embedding
	outputTensor, err := onnx.NewEmptyTensor[float32](outShape)
	must(err)
	defer outputTensor.Destroy()

	session, err := onnx.NewDynamicSession[int64, float32](
		*modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"sentence_embedding"},
	)
	must(err)
	defer session.Destroy()

	// Test timing on inference only, not loading in env + session creation.
	start := time.Now()

	must(session.Run(
		[]*onnx.Tensor[int64]{idTensor, maskTensor},
		[]*onnx.Tensor[float32]{outputTensor},
	))

	elapsed := time.Since(start)

	/* ------------------------------------------------------------------ */
	/* 5. Inspect output ------------------------------------------------ */
	vec := outputTensor.GetData()[:8] // first 8 dims
	j, _ := json.Marshal(vec)

	fmt.Printf("Embedding (first 8 dims): %s …\n", j)
	fmt.Printf("Latency: %v\n", elapsed)
}
