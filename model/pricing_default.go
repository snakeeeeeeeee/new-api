package model

import (
	"strings"
)

type defaultVendorRule struct {
	pattern string
	vendor  string
}

// 视频系列放在通用短名称之前，避免 kling-o3 被归到 OpenAI。
var defaultVendorRules = []defaultVendorRule{
	{pattern: "seedance", vendor: "即梦"},
	{pattern: "jimeng", vendor: "即梦"},
	{pattern: "veo", vendor: "Google"},
	{pattern: "kling", vendor: "快手"},
	{pattern: "doubao", vendor: "字节跳动"},
	{pattern: "vidu", vendor: "Vidu"},
	{pattern: "gpt", vendor: "OpenAI"},
	{pattern: "dall-e", vendor: "OpenAI"},
	{pattern: "whisper", vendor: "OpenAI"},
	{pattern: "o1", vendor: "OpenAI"},
	{pattern: "o3", vendor: "OpenAI"},
	{pattern: "claude", vendor: "Anthropic"},
	{pattern: "gemini", vendor: "Google"},
	{pattern: "moonshot", vendor: "Moonshot"},
	{pattern: "kimi", vendor: "Moonshot"},
	{pattern: "chatglm", vendor: "智谱"},
	{pattern: "glm-", vendor: "智谱"},
	{pattern: "qwen", vendor: "阿里巴巴"},
	{pattern: "deepseek", vendor: "DeepSeek"},
	{pattern: "minimax", vendor: "MiniMax"},
	{pattern: "abab", vendor: "MiniMax"},
	{pattern: "ernie", vendor: "百度"},
	{pattern: "spark", vendor: "讯飞"},
	{pattern: "hunyuan", vendor: "腾讯"},
	{pattern: "command", vendor: "Cohere"},
	{pattern: "@cf/", vendor: "Cloudflare"},
	{pattern: "360", vendor: "360"},
	{pattern: "yi", vendor: "零一万物"},
	{pattern: "jina", vendor: "Jina"},
	{pattern: "mistral", vendor: "Mistral"},
	{pattern: "grok", vendor: "xAI"},
	{pattern: "llama", vendor: "Meta"},
}

func getDefaultVendorName(modelName string) string {
	modelLower := strings.ToLower(modelName)
	for _, rule := range defaultVendorRules {
		if strings.Contains(modelLower, rule.pattern) {
			return rule.vendor
		}
	}
	return ""
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"零一万物":       "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Ollama",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"Vidu":       "Vidu",
	"微软":         "AzureAI",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		// 匹配供应商
		vendorID := 0
		if vendorName := getDefaultVendorName(modelName); vendorName != "" {
			vendorID = getOrCreateVendor(vendorName, vendorMap)
		}

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
