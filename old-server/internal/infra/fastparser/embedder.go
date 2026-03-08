package fastparser

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	onnx "github.com/yalue/onnxruntime_go"
	"github.com/ynqa/wego/pkg/embedding"
	"schma.ai/internal/domain/speech"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/pkg/paths"
)

type Adapter struct {
	// Embedding (ONNX)
	once      sync.Once
	ort       *onnx.DynamicSession[int64, float32]
	outShape  onnx.Shape
	modelPath string
	runtimeSO string
	// Fasttext
	ftOnce  sync.Once            // ensure vec file loads once
	ftModel embedding.Embeddings // in-mem vecs
	synPath string               // path-to.vec
	synOn   bool                 // feature toggle
}

// TODO: Inject adapter in cmd/main, inject paths accordingly.

/**
fp := fastparser.NewAdapter(cfg.ModelDir, cfg.RuntimeDir)

deps := pipeline.Deps{
    STT:  sttRouter,
    FP:   fp,            // now typed as speech.FastParser
    LLM:  llmGemini,
    // …
}
*/

func NewAdapter(modelDir, runtimeDir string, vecPath string, enableSyn bool) *Adapter {
	return &Adapter{
		// Distilbert, Onnx runtime
		modelPath: filepath.Join(modelDir, "bge", "model.int8.onnx"),
		runtimeSO: filepath.Join(runtimeDir, "libonnxruntime.so.1.17.0"),
		outShape:  onnx.NewShape(1, 384),
		// Fasttext synonyms
		synPath: filepath.Join(vecPath, "fasttext", "cc.en.300.100k.vec"),
		synOn:   enableSyn,
	}
}

// initORT initializes the ONNX env and creates a session for the BGE-small model.
func (a *Adapter) initORT() {

	cwd, _ := os.Getwd()
	// baseRuntimePath := paths.RuntimeDir()
	// runtimePath := filepath.Join(baseRuntimePath, "libonnxruntime.so.1.17.0")

	// baseModelPath := paths.ModelDir()
	// bgePath := filepath.Join(baseModelPath, "bge", "model.int8.onnx")

	runtimePath := a.runtimeSO // use injected path
	bgePath := a.modelPath
	logger.Infof("🚀 [NLU] Initializing ONNX Runtime: CWD=%q, RuntimeDir=%q, ModelDir=%q",
		cwd,
		paths.RuntimeDir(),
		paths.ModelDir(),
	)

	if fi, err := os.Stat(runtimePath); err != nil {
		logger.Errorf("❌ [NLU] Unable to stat runtime .so at %s: %v", runtimePath, err)
	} else {
		logger.Infof("✅ [NLU] Found runtime .so (%s), size=%d", runtimePath, fi.Size())
	}

	if fi, err := os.Stat(bgePath); err != nil {
		logger.Errorf("❌ [NLU] Unable to stat ONNX model at %s: %v", bgePath, err)
	} else {
		logger.Infof("✅ [NLU] Found ONNX model (%s), size=%d", bgePath, fi.Size())
	}

	// 1. Set the shared library path
	onnx.SetSharedLibraryPath(runtimePath)
	
	logger.Infof("🚀 [NLU] Set shared library path: %s", runtimePath)

	// 2. Initialize the ONNX environment
	err := onnx.InitializeEnvironment()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize ONNX environment: %v", err))
	}

	logger.Infof("🚀 [NLU] Initialized ONNX environment")

	// 3. Create a session for the BGE-small model (using dynamic session, should I switch to NewAdvnacedDynamicSession)
	session, err := onnx.NewDynamicSession[int64, float32](
		bgePath,
		[]string{"input_ids", "attention_mask"},
		[]string{"sentence_embedding"},
	)

	logger.Infof("✅	[NLU] ONNX Dynamic Session created: session=%#v err=%v", a.ort, err)

	if err != nil {
		logger.Errorf("❌ [NLU] Failed to create ONNX Dynamic session: %v", err)
		panic(err)
	}

	a.ort = session
}

func (a *Adapter) Synonyms(word string, k int) []string {
	return a.NeighbourSynonyms(word, k)
}

// TODO: return error here?
// Embed takes a list of ids and a mask and returns a 384-D embedding.
func (a *Adapter) Embed(ids, mask []int64) []float32 {
	// 1. Initialize the ONNX env and create a session for the BGE-small model. Make sure this only happens once, not on every client connection with sync.Once
	a.once.Do(a.initORT)

	// 2. Create a shape for the input tensor
	shape := onnx.NewShape(1, int64(len(ids)))

	// 3. Create a tensor for the input ids and mask
	idT, _ := onnx.NewTensor[int64](shape, ids)

	// 4. Create a tensor for the input mask
	maskT, _ := onnx.NewTensor[int64](shape, mask)

	// 5. Create a tensor for the output embedding
	outT, _ := onnx.NewEmptyTensor[float32](a.outShape)

	// 6. Run the model
	a.ort.Run([]*onnx.Tensor[int64]{idT, maskT}, []*onnx.Tensor[float32]{outT})

	// 7. Copy the output embedding
	vec := append([]float32(nil), outT.GetData()...)

	// 8. Destroy the tensors (must destroy tensors yourself with dynamic session)
	idT.Destroy()
	maskT.Destroy()
	outT.Destroy()

	// 9. Return the embedding
	return vec
}

func (a *Adapter) Warmup() { a.once.Do(a.initORT) }

// Compile-time assert, run go vet

var _ speech.FastParser = (*Adapter)(nil)
