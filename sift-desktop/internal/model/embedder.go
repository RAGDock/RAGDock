package model

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *tokenizer.Tokenizer
}

func NewEmbedder(modelPath string) (*Embedder, error) {
	// 1. 初始化 ONNX 环境 (保持你之前的修复逻辑)
	if !ort.IsInitialized() {
		// 动态获取路径
		wd, _ := os.Getwd()
		dllPath := filepath.Join(wd, "resources", "lib", "onnxruntime.dll")

		fmt.Printf("🔍 正在动态加载 ONNX DLL: %s\n", dllPath)
		ort.SetSharedLibraryPath(dllPath)

		err := ort.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("ONNX 初始化失败: %v", err)
		}
	}

	// 2. 加载分词器
	tk, err := pretrained.FromFile("resources/models/tokenizer.json")
	if err != nil {
		return nil, fmt.Errorf("分词器加载失败: %v", err)
	}

	// 3. 创建推理会话
	// 注意：输出节点名确认是 "last_hidden_state"
	session, err := ort.NewDynamicAdvancedSession(modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil)
	if err != nil {
		return nil, err
	}

	return &Embedder{
		session:   session,
		tokenizer: tk,
	}, nil
}

func (e *Embedder) Generate(text string) ([]float32, error) {
	en, err := e.tokenizer.EncodeSingle(text, true)
	if err != nil {
		return nil, err
	}

	seqLen := int64(len(en.Ids))
	inputIds := make([]int64, seqLen)
	attentionMask := make([]int64, seqLen)
	tokenTypeIds := make([]int64, seqLen)

	for i := range en.Ids {
		inputIds[i] = int64(en.Ids[i])
		attentionMask[i] = int64(en.AttentionMask[i])
		tokenTypeIds[i] = int64(en.TypeIds[i])
	}

	shape := ort.NewShape(1, seqLen)
	in1, _ := ort.NewTensor[int64](shape, inputIds)
	defer in1.Destroy()
	in2, _ := ort.NewTensor[int64](shape, attentionMask)
	defer in2.Destroy()
	in3, _ := ort.NewTensor[int64](shape, tokenTypeIds)
	defer in3.Destroy()

	outputShape := ort.NewShape(1, seqLen, 384)
	outputTensor, _ := ort.NewEmptyTensor[float32](outputShape)
	defer outputTensor.Destroy()

	err = e.session.Run([]ort.Value{in1, in2, in3}, []ort.Value{outputTensor})
	if err != nil {
		return nil, err
	}

	// --- 🚀 核心改进 1: Mean Pooling (平均池化) ---
	// 将 [1, seqLen, 384] 的张量压缩为 [384] 的向量
	allData := outputTensor.GetData()
	pooledVec := make([]float32, 384)
	var validTokenCount float32 = 0

	for i := 0; i < int(seqLen); i++ {
		// 只对非 Padding 的 Token 进行平均 (依据 attentionMask)
		if attentionMask[i] == 1 {
			validTokenCount++
			for j := 0; j < 384; j++ {
				pooledVec[j] += allData[i*384+j]
			}
		}
	}

	for j := 0; j < 384; j++ {
		pooledVec[j] /= validTokenCount
	}

	// --- 🚀 核心改进 2: L2 Normalization (L2 归一化) ---
	// 确保向量模长为 1，极大提升余弦相似度检索的准确性
	var normSq float32
	for _, v := range pooledVec {
		normSq += v * v
	}
	norm := float32(math.Sqrt(float64(normSq)))
	for i := range pooledVec {
		pooledVec[i] /= norm
	}

	return pooledVec, nil
}
