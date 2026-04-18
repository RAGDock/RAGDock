package model

import (
	"fmt"
	"math"

	"github.com/RAGDock/RAGDock/internal/config"
	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

// Embedder handles local text vectorization using ONNX models
type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *tokenizer.Tokenizer
	Config    *config.AppConfig
}

// NewEmbedder initializes the ONNX runtime and loads the specified model and tokenizer
func NewEmbedder(cfg *config.AppConfig) (*Embedder, error) {
	if !ort.IsInitialized() {
		// Retrieve the platform-specific path for the inference engine library
		ortPath := cfg.GetFullLibPath("libonnxruntime")

		fmt.Printf("Loading inference engine: %s\n", ortPath)
		ort.SetSharedLibraryPath(ortPath)

		// Initialize the ONNX runtime environment
		err := ort.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("ONNX initialization failed: %v", err)
		}
	}

	// Load the pretrained tokenizer from the specified path
	tk, err := pretrained.FromFile(cfg.GetTokenizerPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %v", err)
	}

	// Create an advanced dynamic session for the embedding model
	session, err := ort.NewDynamicAdvancedSession(cfg.GetModelPath(),
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %v", err)
	}

	return &Embedder{
		session:   session,
		tokenizer: tk,
		Config:    cfg,
	}, nil
}

// Generate converts input text into a high-dimensional vector (embedding)
func (e *Embedder) Generate(text string) ([]float32, error) {
	// 1. Tokenize the input text
	en, err := e.tokenizer.EncodeSingle(text, true)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %v", err)
	}

	seqLen := int64(len(en.Ids))
	inputIds := make([]int64, seqLen)
	attentionMask := make([]int64, seqLen)
	tokenTypeIds := make([]int64, seqLen)

	// Cast tokenized values to int64 for ONNX tensors
	for i := range en.Ids {
		inputIds[i] = int64(en.Ids[i])
		attentionMask[i] = int64(en.AttentionMask[i])
		tokenTypeIds[i] = int64(en.TypeIds[i])
	}

	// 2. Prepare ONNX input tensors
	shape := ort.NewShape(1, seqLen)
	in1, _ := ort.NewTensor[int64](shape, inputIds)
	defer in1.Destroy()
	in2, _ := ort.NewTensor[int64](shape, attentionMask)
	defer in2.Destroy()
	in3, _ := ort.NewTensor[int64](shape, tokenTypeIds)
	defer in3.Destroy()

	// 3. Prepare the output tensor with dynamic dimensions
	outputShape := ort.NewShape(1, seqLen, int64(e.Config.ModelDim))
	outputTensor, _ := ort.NewEmptyTensor[float32](outputShape)
	defer outputTensor.Destroy()

	// 4. Execute model inference
	err = e.session.Run([]ort.Value{in1, in2, in3}, []ort.Value{outputTensor})
	if err != nil {
		return nil, fmt.Errorf("inference failed: %v", err)
	}

	// 5. Post-process: Apply mean pooling across valid tokens
	allData := outputTensor.GetData()
	pooledVec := make([]float32, e.Config.ModelDim)
	var validTokenCount float32 = 0

	for i := 0; i < int(seqLen); i++ {
		if attentionMask[i] == 1 {
			validTokenCount++
			for j := 0; j < e.Config.ModelDim; j++ {
				// Offset calculated based on sequence index and model dimensions
				pooledVec[j] += allData[i*e.Config.ModelDim+j]
			}
		}
	}

	// Calculate the average embedding
	for j := 0; j < e.Config.ModelDim; j++ {
		pooledVec[j] /= validTokenCount
	}

	// 6. Perform L2 Normalization for consistent similarity scoring
	var normSq float32
	for _, v := range pooledVec {
		normSq += v * v
	}
	norm := float32(math.Sqrt(float64(normSq)))
	if norm > 0 {
		for i := range pooledVec {
			pooledVec[i] /= norm
		}
	}

	return pooledVec, nil
}
