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
	Answer string `json:"answer"`
}
