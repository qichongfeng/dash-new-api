package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// 公告面向外部应用（如钱喵记账小程序）：公开接口免登录、按 app 参数隔离，
// 管理接口仅供本控制台 root 使用。

var validAnnouncementTypes = map[string]bool{
	"default": true,
	"ongoing": true,
	"success": true,
	"warning": true,
	"error":   true,
}

// GetAnnouncementList 公开列表（免登录，不含正文）
func GetAnnouncementList(c *gin.Context) {
	app := strings.TrimSpace(c.Query("app"))
	if app == "" {
		common.ApiErrorMsg(c, "app 参数不能为空")
		return
	}
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.GetPublicAnnouncements(app, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// GetAnnouncementDetail 公开详情（免登录，含正文；未启用/未发布视为不存在）
func GetAnnouncementDetail(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a, err := model.GetPublicAnnouncementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if a == nil {
		common.ApiErrorMsg(c, "公告不存在")
		return
	}
	common.ApiSuccess(c, a)
}

// ---- Admin APIs ----

func AdminListAnnouncements(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.GetAdminAnnouncements(strings.TrimSpace(c.Query("app")), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

type AdminUpsertAnnouncementRequest struct {
	App         string `json:"app"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	LinkURL     string `json:"link_url"` // 可选；有值时客户端点击直接打开该链接
	Type        string `json:"type"`
	PublishTime int64  `json:"publish_time"` // 0 = 立即发布
	Enabled     *bool  `json:"enabled"`
}

func validateAnnouncement(app, title, content, linkURL, typ string) string {
	if app == "" {
		return "应用标识不能为空"
	}
	if len([]rune(app)) > 32 {
		return "应用标识不能超过32字"
	}
	if title == "" {
		return "公告标题不能为空"
	}
	if len([]rune(title)) > 128 {
		return "公告标题不能超过128字"
	}
	if len([]rune(content)) > 10000 {
		return "公告内容不能超过10000字"
	}
	if linkURL != "" {
		if !strings.HasPrefix(linkURL, "http://") && !strings.HasPrefix(linkURL, "https://") {
			return "链接必须以 http:// 或 https:// 开头"
		}
		if len([]rune(linkURL)) > 512 {
			return "链接不能超过512字"
		}
	}
	if !validAnnouncementTypes[typ] {
		return "公告类型非法"
	}
	return ""
}

// normalizeAnnouncementRequest 整理入参并校验，返回错误提示（空串=通过）
func normalizeAnnouncementRequest(req *AdminUpsertAnnouncementRequest) string {
	req.App = strings.TrimSpace(req.App)
	req.Title = strings.TrimSpace(req.Title)
	req.LinkURL = strings.TrimSpace(req.LinkURL)
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		req.Type = "default"
	}
	return validateAnnouncement(req.App, req.Title, req.Content, req.LinkURL, req.Type)
}

func AdminCreateAnnouncement(c *gin.Context) {
	var req AdminUpsertAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if msg := normalizeAnnouncementRequest(&req); msg != "" {
		common.ApiErrorMsg(c, msg)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	publishTime := req.PublishTime
	if publishTime <= 0 {
		publishTime = common.GetTimestamp()
	}
	a := model.Announcement{
		App:         req.App,
		Title:       req.Title,
		Content:     req.Content,
		LinkURL:     req.LinkURL,
		Type:        req.Type,
		PublishTime: publishTime,
		Enabled:     enabled,
	}
	if err := model.DB.Create(&a).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, a)
}

func AdminUpdateAnnouncement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	existing, err := model.GetAnnouncementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing == nil {
		common.ApiErrorMsg(c, "公告不存在")
		return
	}
	var req AdminUpsertAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if msg := normalizeAnnouncementRequest(&req); msg != "" {
		common.ApiErrorMsg(c, msg)
		return
	}
	publishTime := req.PublishTime
	if publishTime <= 0 {
		publishTime = common.GetTimestamp()
	}
	existing.App = req.App
	existing.Title = req.Title
	existing.Content = req.Content
	existing.LinkURL = req.LinkURL
	existing.Type = req.Type
	existing.PublishTime = publishTime
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := model.DB.Save(existing).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, existing)
}

type AdminUpdateAnnouncementStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

func AdminUpdateAnnouncementStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpdateAnnouncementStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.DB.Model(&model.Announcement{}).Where("id = ?", id).Update("enabled", *req.Enabled).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminDeleteAnnouncement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if err := model.DB.Delete(&model.Announcement{}, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
