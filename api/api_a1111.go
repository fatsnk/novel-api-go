package api

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"novel-api/config"
	"novel-api/models"
	"time"
)

// A1111Txt2ImgRequest 兼容 A1111 sdapi/v1/txt2img 的请求结构
type A1111Txt2ImgRequest struct {
	Prompt           string                 `json:"prompt"`
	NegativePrompt   string                 `json:"negative_prompt,omitempty"`
	Steps            int                    `json:"steps,omitempty"`
	Width            int                    `json:"width,omitempty"`
	Height           int                    `json:"height,omitempty"`
	SamplerName      string                 `json:"sampler_name,omitempty"`
	CfgScale         float64                `json:"cfg_scale,omitempty"`
	Seed             int                    `json:"seed,omitempty"`
	BatchSize        int                    `json:"batch_size,omitempty"`
	OverrideSettings map[string]interface{} `json:"override_settings,omitempty"`
	// 可根据需要添加其他 A1111 支持的参数，当前仅提取影响NovelAI生成的关键参数
}

// A1111Txt2Img 处理 A1111 兼容的文生图请求 (无验证)
func A1111Txt2Img(w http.ResponseWriter, r *http.Request, cfg *config.Config) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// 如果是 OPTIONS 请求，直接返回 200 OK
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. 解析请求体
	var req A1111Txt2ImgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode A1111 request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("A1111 Txt2Img request: Prompt=%s", req.Prompt)

	// 2. 处理参数默认值，如果没有提供，则使用配置文件的默认值
	width := cfg.Parameters.Width
	if req.Width > 0 {
		width = req.Width
	}

	height := cfg.Parameters.Height
	if req.Height > 0 {
		height = req.Height
	}

	steps := cfg.Parameters.Steps
	if req.Steps > 0 {
		steps = req.Steps
	}

	cfgScale := cfg.Parameters.Scale
	if req.CfgScale > 0 {
		cfgScale = req.CfgScale
	}

	seed := req.Seed
	if seed <= -1 {
		rand.Seed(time.Now().UnixNano())
		seed = rand.Intn(1000000)
	}

	// 3. 构造给 NovelAI 模型的临时配置对象（为了不覆盖全局配置，这里可以临时修改）
	// 注意：因为我们目前使用的模型生成函数直接接收 cfg 参数，所以我们先深拷贝一个 cfg 副本用于本次请求
	tempCfg := *cfg
	tempCfg.Parameters.Width = width
	tempCfg.Parameters.Height = height
	tempCfg.Parameters.Steps = steps
	tempCfg.Parameters.Scale = cfgScale
	
	// 如果提供了负面提示词，覆盖默认的
	if req.NegativePrompt != "" {
		tempCfg.Parameters.CustomAntiWords = req.NegativePrompt
	}

	// 4. 获取用户输入的提示词
	userInput := req.Prompt

	// 5. 提取图片链接(可选，A1111的txt2img一般不带，如果有按原逻辑处理)
	imageURL := extractLinksFromPrompt(userInput)
	var base64String string
	if len(imageURL) > 0 {
		imageURLS := imageURL[0]
		base64String, _ = ImageURLToBase64(imageURLS)
	}

	// 6. 确定使用的模型
	modelName := "nai-diffusion-3" // 默认模型
	if override, ok := req.OverrideSettings["sd_model_checkpoint"]; ok {
		if modelStr, ok := override.(string); ok && modelStr != "" {
			modelName = modelStr
		}
	}

	// 构建兼容的 ChatRequest 结构
	compatibleReq := config.ChatRequest{
		Authorization: tempCfg.NovelAI.Key, // A1111无验证，使用全局配置的默认Key
		Model:         modelName,
		Messages: []config.Message{
			{
				Role:    "user",
				Content: userInput,
			},
		},
	}

	// 7. 标识这是 DALL-E 格式请求 (DALL-E格式的响应包含 JSON {"data":[{"url":"..."}]}，与部分A1111兼容插件接受的格式类似或方便解析)
	// 其实 A1111 标准返回是 {"images": ["base64..."], "parameters": {}, "info": "{}"}
	// 为了最快接入并由于我们已经有上传器返回了 URL，如果调用方接受 URL，我们可以先返回类似结构，
	// 如果必须返回 Base64，我们可能需要修改模型函数以返回 Base64 而非上传。
	// 这里先复用 isDallRequest=true 的处理逻辑，即返回 JSON URL 响应。
	
	// 因为我们没有提供原生的A1111完整返回格式（全是base64），我们借助 isDallRequest 来返回JSON对象。
	// 但实际上 A1111 api 需要返回 {"images": ["base64string"]}，我们现在的 Nai3WithFormatAndSize 默认上传并返回 URL。
	// 如果客户端能接受 URL 尚可，严格的A1111客户端需要 images 数组包含 base64。
	// 这里我们还是复用现有的 isDallRequest 逻辑（返回 URL），如果后续客户端不兼容，可以在 models 中增加跳过上传直接返回 base64 的逻辑。
	isDallRequest := true

	// 调用对应模型的生成函数
	switch modelName {
	case "nai-diffusion-3":
		models.Nai3WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, width, height, isDallRequest)
	case "nai-diffusion-furry-3":
		models.Nai3WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, width, height, isDallRequest)
	case "nai-diffusion-4-full":
		models.Nai4WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, nil, width, height, isDallRequest)
	case "nai-diffusion-4-curated-preview":
		models.Nai4WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, nil, width, height, isDallRequest)
	case "nai-diffusion-4-5-curated":
		models.Nai4WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, nil, width, height, isDallRequest)
	case "nai-diffusion-4-5-full":
		models.Nai4WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, nil, width, height, isDallRequest)
	default:
		// 如果匹配不到，降级为默认模型
		log.Printf("Unknown A1111 model override '%s', falling back to nai-diffusion-3", modelName)
		compatibleReq.Model = "nai-diffusion-3"
		models.Nai3WithFormatAndSize(w, r, compatibleReq, seed, base64String, tempCfg.NovelAI.Key, &tempCfg, userInput, width, height, isDallRequest)
	}
}

// A1111Models 处理 A1111 兼容的模型列表请求
func A1111Models(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 返回我们支持的模型列表
	models := []map[string]interface{}{
		{
			"title":      "nai-diffusion-3",
			"model_name": "nai-diffusion-3",
			"hash":       "nai3",
			"sha256":     "nai3",
			"filename":   "nai-diffusion-3",
		},
		{
			"title":      "nai-diffusion-4-full",
			"model_name": "nai-diffusion-4-full",
			"hash":       "nai4",
			"sha256":     "nai4",
			"filename":   "nai-diffusion-4-full",
		},
		{
			"title":      "nai-diffusion-furry-3",
			"model_name": "nai-diffusion-furry-3",
			"hash":       "furry3",
			"sha256":     "furry3",
			"filename":   "nai-diffusion-furry-3",
		},
		{
			"title":      "nai-diffusion-4-curated-preview",
			"model_name": "nai-diffusion-4-curated-preview",
			"hash":       "nai4cp",
			"sha256":     "nai4cp",
			"filename":   "nai-diffusion-4-curated-preview",
		},
		{
			"title":      "nai-diffusion-4-5-curated",
			"model_name": "nai-diffusion-4-5-curated",
			"hash":       "nai45c",
			"sha256":     "nai45c",
			"filename":   "nai-diffusion-4-5-curated",
		},
		{
			"title":      "nai-diffusion-4-5-full",
			"model_name": "nai-diffusion-4-5-full",
			"hash":       "nai45f",
			"sha256":     "nai45f",
			"filename":   "nai-diffusion-4-5-full",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

// A1111Progress 处理 A1111 兼容的进度查询请求
func A1111Progress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 因为此代理使用的是同步阻塞请求，无法获取 NovelAI 的实时生成进度，返回空闲状态的默认数据
	progress := map[string]interface{}{
		"progress":     0.0,
		"eta_relative": 0.0,
		"state": map[string]interface{}{
			"skipped":        false,
			"interrupted":    false,
			"job":            "",
			"job_count":      0,
			"job_timestamp":  "19700101000000",
			"job_no":         0,
			"sampling_step":  0,
			"sampling_steps": 0,
		},
		"current_image": nil,
		"textinfo":      nil,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}
