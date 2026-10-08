package controller

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// 公告图片：管理员上传落盘到静态目录（不进数据库），公开路由读取。
// 文件名全部由服务端生成（32 位 hex + 白名单扩展名），读取时按同形态二次校验，杜绝路径穿越。

const maxAnnouncementImageBytes = 5 << 20

// 白名单同时给出 mime 对应的落盘扩展名；不含 svg（公告图片无需矢量，少一个脚本向量）
var allowedAnnouncementImageMimeTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// announcementImageNameRe 服务端生成的文件名形态，读取时按此白名单校验
var announcementImageNameRe = regexp.MustCompile(`^[a-f0-9]{32}\.(png|jpe?g|gif|webp)$`)

type AdminUploadAnnouncementImageRequest struct {
	Image string `json:"image"` // data:image/png;base64,...
}

// decodeAnnouncementImageDataURI 解析并校验 data URI：声明的 mime 与魔数检测都须落在白名单内，
// 以魔数检测结果为准返回（防伪装）。errMsg 非空表示拒绝。
func decodeAnnouncementImageDataURI(dataURI string) (mimeType string, data []byte, errMsg string) {
	head, payload, ok := strings.Cut(dataURI, ",")
	if !ok {
		return "", nil, "图片必须为 data:image/...;base64 格式"
	}
	mimePart, ok := strings.CutPrefix(head, "data:")
	if !ok {
		return "", nil, "图片必须为 data:image/...;base64 格式"
	}
	declared, ok := strings.CutSuffix(mimePart, ";base64")
	if !ok {
		return "", nil, "图片必须为 data:image/...;base64 格式"
	}
	if _, allowed := allowedAnnouncementImageMimeTypes[declared]; !allowed {
		return "", nil, "图片必须为 PNG、JPG、GIF 或 WebP 格式"
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(payload)
	if err != nil {
		return "", nil, "图片 base64 解码失败"
	}
	if len(decoded) > maxAnnouncementImageBytes {
		return "", nil, "图片不能超过5MB"
	}
	detected := http.DetectContentType(decoded)
	if _, allowed := allowedAnnouncementImageMimeTypes[detected]; !allowed {
		return "", nil, "图片内容不是有效的 PNG、JPG、GIF 或 WebP"
	}
	return detected, decoded, ""
}

// AdminUploadAnnouncementImage 上传公告图片，返回可直接嵌入 Markdown 的相对 src
func AdminUploadAnnouncementImage(c *gin.Context) {
	var req AdminUploadAnnouncementImageRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Image) == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	mimeType, data, errMsg := decodeAnnouncementImageDataURI(req.Image)
	if errMsg != "" {
		common.ApiErrorMsg(c, errMsg)
		return
	}
	var seed [16]byte
	if _, err := rand.Read(seed[:]); err != nil {
		common.ApiError(c, err)
		return
	}
	name := hex.EncodeToString(seed[:]) + "." + allowedAnnouncementImageMimeTypes[mimeType]
	if err := os.WriteFile(filepath.Join(*common.AnnouncementImageDir, name), data, 0644); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"src": "/api/announcement/images/" + name})
}

// GetAnnouncementImage 公开读取公告图片（小程序 <image> 与管理台预览直接引用）
func GetAnnouncementImage(c *gin.Context) {
	name := c.Param("name")
	if !announcementImageNameRe.MatchString(name) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	path := filepath.Join(*common.AnnouncementImageDir, name)
	if _, err := os.Stat(path); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	// 内容按 id 不可变，允许公开缓存；nosniff 防嗅探（Content-Type 由扩展名决定）
	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(path)
}
