package controller

import (
	"encoding/base64"
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

const (
	announcementListDefaultLimit = 20
	announcementListMaxLimit     = 50
)

// encodeAnnouncementCursor 游标 = 上一页末条 (publish_time, id) 的 base64url("publish_time:id")，
// 与列表排序键 (publish_time DESC, id DESC) 对应
func encodeAnnouncementCursor(publishTime int64, id int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(publishTime, 10) + ":" + strconv.Itoa(id)))
}

func decodeAnnouncementCursor(cursor string) (publishTime int64, id int, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, 0, err
	}
	ptStr, idStr, ok := strings.Cut(string(raw), ":")
	if !ok {
		return 0, 0, strconv.ErrSyntax
	}
	publishTime, err = strconv.ParseInt(ptStr, 10, 64)
	if err != nil {
		return 0, 0, err
	}
	id, err = strconv.Atoi(idStr)
	if err != nil {
		return 0, 0, err
	}
	if publishTime <= 0 || id <= 0 {
		return 0, 0, strconv.ErrSyntax
	}
	return publishTime, id, nil
}

// GetAnnouncementList 公开列表（免登录，不含正文），按 (publish_time, id) 倒序游标翻页
func GetAnnouncementList(c *gin.Context) {
	app := strings.TrimSpace(c.Query("app"))
	if app == "" {
		common.ApiErrorMsg(c, "app 参数不能为空")
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = announcementListDefaultLimit
	}
	limit = min(limit, announcementListMaxLimit)
	var beforePublishTime int64
	var beforeId int
	if cursor := strings.TrimSpace(c.Query("cursor")); cursor != "" {
		pt, id, err := decodeAnnouncementCursor(cursor)
		if err != nil {
			common.ApiErrorMsg(c, "cursor 参数非法")
			return
		}
		beforePublishTime, beforeId = pt, id
	}
	items, err := model.GetPublicAnnouncementPage(app, limit, beforePublishTime, beforeId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 拿满一页才可能还有下一页；末页 next_cursor 为空串
	nextCursor := ""
	if len(items) == limit {
		last := items[len(items)-1]
		nextCursor = encodeAnnouncementCursor(last.PublishTime, last.Id)
	}
	common.ApiSuccess(c, gin.H{"items": items, "next_cursor": nextCursor})
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
	Content     string `json:"content"` // Markdown 原文
	Type        string `json:"type"`
	PublishTime int64  `json:"publish_time"` // 0 = 立即发布
	Enabled     *bool  `json:"enabled"`
}

func validateAnnouncement(app, title, content, typ string) string {
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
	if !validAnnouncementTypes[typ] {
		return "公告类型非法"
	}
	return ""
}

// normalizeAnnouncementRequest 整理入参并校验，返回错误提示（空串=通过）
func normalizeAnnouncementRequest(req *AdminUpsertAnnouncementRequest) string {
	req.App = strings.TrimSpace(req.App)
	req.Title = strings.TrimSpace(req.Title)
	req.Type = strings.TrimSpace(req.Type)
	if req.Type == "" {
		req.Type = "default"
	}
	return validateAnnouncement(req.App, req.Title, req.Content, req.Type)
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
