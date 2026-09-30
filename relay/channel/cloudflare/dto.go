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
type CfVisionResponse struct {
	Result CfVisionResult `json:"result"`
}

type CfVisionResult struct {
	Answer string `json:"answer"`
}
