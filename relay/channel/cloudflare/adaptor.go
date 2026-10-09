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
	"github.com/QuantumNous/new-api/service"

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

// CF 视觉协议取值：渠道设置 vision_protocols 的 value
const (
	cfVisionProtocolMoondream = "moondream"
	cfVisionProtocolLlama     = "llama-vision"
)

// cfVisionProtocol 查渠道设置的 vision_protocols 表，返回该上游模型的视觉协议：
// ""（未配置）= 普通模型走 OpenAI 兼容端点；moondream / llama-vision = 该协议的
// 原生 /ai/run/ 转换。模型与协议的对应关系完全由渠道配置决定，适配层不内置
// 模型清单（Cloudflare 的 OpenAI 兼容端点不支持 image_url，这些模型必须转原生）。
func cfVisionProtocol(info *relaycommon.RelayInfo) string {
	return info.ChannelSetting.VisionProtocols[info.UpstreamModelName]
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch info.RelayMode {
	case constant.RelayModeChatCompletions:
		if cfVisionProtocol(info) != "" {
			break // 视觉协议模型：落到下方原生 /ai/run/
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
		// 视觉协议模型（渠道设置 vision_protocols 声明的）：messages + image_url
		// 按配置的协议转原生 /ai/run/ 输入
		if info.RelayMode == constant.RelayModeChatCompletions {
			switch cfVisionProtocol(info) {
			case cfVisionProtocolLlama:
				return convertCfLlamaVisionRequest(request)
			case cfVisionProtocolMoondream:
				return convertCfVisionRequest(request)
			}
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
		"task":     "query",
		"image":    image,
		"question": strings.Join(texts, "\n"),
		// reasoning 不传、走官方默认 true：曾用显式 false 省 token，但 moondream3.1
		// 上线后 false 路径出现 success:true + answer 空 + in_tokens=0（图未进模型）
		// 的空响应，回到官方默认路径最稳
		"stream": false, // 文档参数页两处默认值不一致(true/false)，显式关闭走一次性 JSON 响应
	}, nil
}

// maxCfLlamaVisionImageBytes 图转 JSON 字节数组后体积膨胀 ~4 倍，压一道上限
const maxCfLlamaVisionImageBytes = 5 << 20

// convertCfLlamaVisionRequest llama-3.2-vision 原生输入：{prompt, image:[字节]}。
// image_url 支持两种来源：data URI（小程序拍照）直接解码；公网 URL 由服务端代下。
func convertCfLlamaVisionRequest(request *dto.GeneralOpenAIRequest) (any, error) {
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
	var raw []byte
	if dataURI, ok := strings.CutPrefix(image, "data:"); ok {
		_, payload, found := strings.Cut(dataURI, ",")
		if !found {
			return nil, errors.New("vision request image data URI is malformed")
		}
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, errors.New("vision request image data URI is not valid base64")
		}
		raw = decoded
	} else if strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		_, b64, err := service.GetImageFromUrl(image)
		if err != nil {
			return nil, fmt.Errorf("vision request image download failed: %w", err)
		}
		raw, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, errors.New("vision request image download yielded invalid base64")
		}
	} else {
		return nil, errors.New("vision request image must be a data URI or an http(s) URL")
	}
	if len(raw) == 0 {
		return nil, errors.New("vision request image is empty")
	}
	if len(raw) > maxCfLlamaVisionImageBytes {
		return nil, fmt.Errorf("vision request image exceeds %d bytes", maxCfLlamaVisionImageBytes)
	}
	// []byte 会被 JSON 编成 base64 字符串，必须转数字切片才是官方要的字节数组
	ints := make([]int, len(raw))
	for i, b := range raw {
		ints[i] = int(b)
	}
	return map[string]any{
		"prompt": strings.Join(texts, "\n"),
		"image":  ints,
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
		case cfVisionProtocol(info) != "":
			// 视觉协议模型：原生 /ai/run/ 响应转 chat completion，先于流式判断
			//（这些模型不支持流式，误入 cfStreamHandler 会解析不出内容）
			if cfVisionProtocol(info) == cfVisionProtocolLlama {
				err, usage = cfLlamaVisionHandler(c, info, resp)
			} else {
				err, usage = cfVisionHandler(c, info, resp)
			}
		case info.IsStream:
			err, usage = cfStreamHandler(c, info, resp)
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
