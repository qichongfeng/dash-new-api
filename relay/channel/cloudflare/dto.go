package cloudflare

import "github.com/QuantumNous/new-api/relaykit/dto"

type CfRequest struct {
	Messages    []dto.Message `json:"messages,omitempty"`
	Lora        string        `json:"lora,omitempty"`
	MaxTokens   uint          `json:"max_tokens,omitempty"`
	Prompt      string        `json:"prompt,omitempty"`
	Raw         bool          `json:"raw,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type CfAudioResponse struct {
	Result CfSTTResult `json:"result"`
}

type CfSTTResult struct {
	Text string `json:"text"`
}

// CfVisionResponse /ai/run/<moondream>（Image-to-Text）的响应，query 任务取 answer。
// CF REST 出错时返回 {"success":false,"errors":[...]} 信封（无 result 字段），
// success/errors 用于把上游真实错误透传给客户端，而不是吞成 200 空 content。
type CfVisionResponse struct {
	Result  CfVisionResult `json:"result"`
	Success bool           `json:"success"`
	Errors  []CfApiError   `json:"errors"`
}

type CfApiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CfVisionResult struct {
	Answer       string            `json:"answer"`
	Caption      string            `json:"caption"`
	FinishReason string            `json:"finish_reason"`
	Reasoning    CfVisionReasoning `json:"reasoning"`
	Metrics      CfVisionMetrics   `json:"metrics"`
}

// CfVisionReasoning query 任务 reasoning=true 时的推理链文本
type CfVisionReasoning struct {
	Text string `json:"text"`
}

// CfVisionMetrics 官方输出 schema 的 metrics：真实 token 消耗（input 小 → 图没被计入）
type CfVisionMetrics struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// CfLlamaVisionResponse llama-3.2-vision 原生 /ai/run/ 响应：正文在 result.response，
// usage 为上游真实计量（prompt_tokens 含图）。错误信封与 moondream 相同。
type CfLlamaVisionResponse struct {
	Result  CfLlamaVisionResult `json:"result"`
	Success bool                `json:"success"`
	Errors  []CfApiError        `json:"errors"`
}

type CfLlamaVisionResult struct {
	// Response 多态：普通问答是字符串；提示词要求 JSON 输出时 CF 会把模型输出
	// 解析成对象直接放这里（实测 {"amount":0,...}），handler 统一转回文本
	Response any                `json:"response"`
	Usage    CfLlamaVisionUsage `json:"usage"`
}

type CfLlamaVisionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}
