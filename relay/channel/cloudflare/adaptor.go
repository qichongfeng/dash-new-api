package cloudflare

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

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

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch info.RelayMode {
	case constant.RelayModeChatCompletions:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/chat/completions", info.ChannelBaseUrl, info.ApiVersion), nil
	case constant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/embeddings", info.ChannelBaseUrl, info.ApiVersion), nil
	case constant.RelayModeResponses:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/responses", info.ChannelBaseUrl, info.ApiVersion), nil
	default:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/run/%s", info.ChannelBaseUrl, info.ApiVersion, info.UpstreamModelName), nil
	}
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
		return request, nil
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	// 转写走 DoFormRequest：multipart 请求需要带上 ConvertAudioRequest 里写入的 Content-Type（含 boundary）
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
	// CF /ai/run/<whisper> 按 multipart（file 字段）收音频：重建 multipart 而不是透传裸字节，
	// 并把带 boundary 的 Content-Type 写回 c.Request（DoFormRequest 会复制它）。
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}
	fileHeaders := formData.File["file"]
	if len(fileHeaders) == 0 {
		return nil, errors.New("file is required")
	}
	fileHeader := fileHeaders[0]
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %v", err)
	}
	defer file.Close()
	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return nil, errors.New("create form file failed")
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, errors.New("copy file failed")
	}
	writer.Close()
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return &requestBody, nil
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
		if info.IsStream {
			err, usage = cfStreamHandler(c, info, resp)
		} else {
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
