package cloudflare

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

// isCfVisionModel CF 上的 Image-to-Text 模型（moondream 系）：OpenAI 兼容 chat 端点
// 不支持它们（上游直接 400），需转原生 /ai/run/{task,image,question}。
func isCfVisionModel(upstreamModelName string) bool {
	return strings.HasPrefix(upstreamModelName, "@cf/moondream/")
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch info.RelayMode {
	case constant.RelayModeChatCompletions:
		if isCfVisionModel(info.UpstreamModelName) {
			break // moondream：落到下方原生 /ai/run/
		}
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/chat/completions", info.ChannelBaseUrl, info.ApiVersion), nil
	case constant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/embeddings", info.ChannelBaseUrl, info.ApiVersion), nil
	case constant.RelayModeResponses:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/responses", info.ChannelBaseUrl, info.ApiVersion), nil
	}
	return fmt.Sprintf("%s/client/v4/accounts/%s/ai/run/%s", info.ChannelBaseUrl, info.ApiVersion, info.UpstreamModelName), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch info.RelayMode {
	case constant.RelayModeCompletions:
		return convertCf2CompletionsRequest(*request), nil
	default:
		// moondream 系（Image-to-Text）：messages + image_url 转原生 {task,image,question}
		if info.RelayMode == constant.RelayModeChatCompletions && isCfVisionModel(info.UpstreamModelName) {
			return convertCfVisionRequest(request)
		}
		return request, nil
	}
}

// convertCfVisionRequest chat 视觉请求 → moondream 原生输入：task=query，
// image 取第一个 image_url（CF 接受公共 HTTPS URL 或 base64 data URI），
// question 为全部文本内容拼接（system/多段 text 合成一个提问）。
func convertCfVisionRequest(request *dto.GeneralOpenAIRequest) (any, error) {
	var image string
	texts := make([]string, 0, 2)
	for i := range request.Messages {
		for _, item := range request.Messages[i].ParseContent() {
			if item.Type == "image_url" {
				if img := item.GetImageMedia(); img != nil && image == "" {
					image = img.Url
				}
			} else if item.Type == "text" && item.Text != "" {
				texts = append(texts, item.Text)
			}
		}
	}
	if image == "" {
		return nil, errors.New("vision request requires an image_url")
	}
	return map[string]any{
		"task":      "query",
		"image":     image,
		"question":  strings.Join(texts, "\n"),
		"reasoning": false, // 官方默认 true（先出推理链）：结构化抽取只要 answer，省 token 且避开 answer 为空的怪癖
		"stream":    false, // 文档参数页两处默认值不一致(true/false)，显式关闭走一次性 JSON 响应
	}, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	// 转写走 DoFormRequest：它会把 ConvertAudioRequest 写入 c.Request 的 Content-Type
	// （这里是 application/json）原样复制给上游请求；DoApiRequest 的 audio 分支不带 Content-Type。
	if info.RelayMode == constant.RelayModeAudioTranscription ||
		info.RelayMode == constant.RelayModeAudioTranslation {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	// CF Workers AI /ai/run/<whisper> 按 JSON 收音频：{"audio":"<base64>", "language":"zh"?}
	//（官方 curl：-d '{"audio":"'$AUDIO_BASE64'"}'）。读取客户端 multipart 的 file 转 base64，
	// Content-Type 写回 c.Request，经 DoFormRequest 复制给上游请求。
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}
	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return nil, errors.New("file is required")
	}
	file, err := fileHeaders[0].Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %v", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.New("read audio file failed")
	}
	payload := map[string]any{"audio": base64.StdEncoding.EncodeToString(raw)}
	if langs := formData.Value["language"]; len(langs) > 0 && langs[0] != "" {
		payload["language"] = langs[0]
	}
	jsonBody, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	c.Request.Header.Set("Content-Type", "application/json")
	return bytes.NewReader(jsonBody), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case constant.RelayModeEmbeddings:
		fallthrough
	case constant.RelayModeChatCompletions:
		switch {
		case info.IsStream:
			err, usage = cfStreamHandler(c, info, resp)
		case isCfVisionModel(info.UpstreamModelName):
			// moondream：原生 /ai/run/ 响应转 chat completion（客户端视觉调用为非流式）
			err, usage = cfVisionHandler(c, info, resp)
		default:
			err, usage = cfHandler(c, info, resp)
		}
	case constant.RelayModeResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesHandler(c, info, resp)
		}
	case constant.RelayModeAudioTranslation:
		fallthrough
	case constant.RelayModeAudioTranscription:
		err, usage = cfSTTHandler(c, info, resp)
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
