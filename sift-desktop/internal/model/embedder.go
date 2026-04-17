package model

import (
	"fmt"
	"math"

	"sift.local/internal/config" // 引入配置包

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
	ort "github.com/yalue/onnxruntime_go"
)

type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *tokenizer.Tokenizer
	Config    *config.AppConfig
}

func NewEmbedder(cfg *config.AppConfig) (*Embedder, error) {
	if !ort.IsInitialized() {
		// 跨平台获取 onnxruntime 路径
		ortPath := cfg.GetFullLibPath("onnxruntime")

		fmt.Printf("🔍 正在加载推理引擎: %s\n", ortPath)
		ort.SetSharedLibraryPath(ortPath)
		_ = ort.InitializeEnvironment()
	}

	// 使用配置中的分词器路径
	tk, err := pretrained.FromFile(cfg.GetTokenizerPath())
	if err != nil {
		return nil, fmt.Errorf("分词器加载失败: %v", err)
	}

	// 使用配置中的模型路径
	session, err := ort.NewDynamicAdvancedSession(cfg.GetModelPath(),
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		nil)
	if err != nil {
		return nil, err
	}

	return &Embedder{
		session:   session,
		tokenizer: tk,
		Config:    cfg,
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

	// 动态维度输出
	outputShape := ort.NewShape(1, seqLen, int64(e.Config.ModelDim))
	outputTensor, _ := ort.NewEmptyTensor[float32](outputShape)
	defer outputTensor.Destroy()

	err = e.session.Run([]ort.Value{in1, in2, in3}, []ort.Value{outputTensor})
	if err != nil {
		return nil, err
	}

	allData := outputTensor.GetData()
	pooledVec := make([]float32, e.Config.ModelDim)
	var validTokenCount float32 = 0

	for i := 0; i < int(seqLen); i++ {
		if attentionMask[i] == 1 {
			validTokenCount++
			for j := 0; j < e.Config.ModelDim; j++ {
				// 正确处理动态偏移量
				pooledVec[j] += allData[i*e.Config.ModelDim+j]
			}
		}
	}

	for j := 0; j < e.Config.ModelDim; j++ {
		pooledVec[j] /= validTokenCount
	}

	// L2 归一化逻辑保持一致...
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
