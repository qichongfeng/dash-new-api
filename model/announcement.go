package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Announcement 面向外部应用（如钱喵记账小程序）的公告，按 App 维度隔离
type Announcement struct {
	Id int `json:"id"`

	App   string `json:"app" gorm:"type:varchar(32);not null;index"` // 应用标识，如 qianmiao
	Title string `json:"title" gorm:"type:varchar(128);not null"`
	// Markdown 原文（客户端自行解析渲染；图片以相对路径 /api/announcement/images/... 引用）
	Content string `json:"content" gorm:"type:text"`
	// 沿用控制台公告的 5 类样式：default|ongoing|success|warning|error
	Type string `json:"type" gorm:"type:varchar(16);not null;default:'default'"`
	// unix 秒；0 = 创建即视为发布；大于当前时间为定时发布（公开接口不返回）
	PublishTime int64 `json:"publish_time" gorm:"bigint;not null;default:0"`
	// 默认值由 controller 创建逻辑显式设置（bool 的 default 标签在 MySQL/PG
	// 上会因布尔归一化差异导致 AutoMigrate 反复 ALTER）
	Enabled bool `json:"enabled"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (Announcement) TableName() string {
	return "announcements"
}

func (a *Announcement) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

func (a *Announcement) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = common.GetTimestamp()
	return nil
}

// AnnouncementSummary 公开列表项（不含 Content 正文，省流量）
type AnnouncementSummary struct {
	Id          int    `json:"id"`
	App         string `json:"app"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	PublishTime int64  `json:"publish_time"`
	CreatedAt   int64  `json:"created_at"`
}

// AdminAnnouncementSummary 管理列表项：同样不带 Content（正文走详情接口），
// 额外带管理面需要的状态与更新时间
type AdminAnnouncementSummary struct {
	AnnouncementSummary
	Enabled   bool  `json:"enabled"`
	UpdatedAt int64 `json:"updated_at"`
}

// GetPublicAnnouncementPage 对外可见的公告（启用且已发布），按 (publish_time, id) 倒序游标翻页。
// beforeId > 0 表示带上 cursor 的 keyset 条件（取该条之前的数据），无 offset 翻页漂移。
func GetPublicAnnouncementPage(app string, limit int, beforePublishTime int64, beforeId int) ([]AnnouncementSummary, error) {
	query := DB.Model(&Announcement{}).
		Where("app = ? AND enabled = ? AND publish_time <= ?", app, true, common.GetTimestamp())
	if beforeId > 0 {
		// 整体加括号：避免与外层 AND 拼接时 OR 抬升优先级
		query = query.Where("(publish_time < ? OR (publish_time = ? AND id < ?))", beforePublishTime, beforePublishTime, beforeId)
	}
	var items []AnnouncementSummary
	err := query.
		Select("id, app, title, type, publish_time, created_at").
		Order("publish_time DESC, id DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// GetPublicAnnouncementById 公开详情：未启用/未发布一律视为不存在
func GetPublicAnnouncementById(id int) (*Announcement, error) {
	if id <= 0 {
		return nil, nil
	}
	var a Announcement
	err := DB.Where("id = ? AND enabled = ? AND publish_time <= ?", id, true, common.GetTimestamp()).
		First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAdminAnnouncements 管理列表：app 为空时返回全部应用，含停用与未发布；
// 不查 Content 正文（编辑时走 GetAnnouncementById 详情）
func GetAdminAnnouncements(app string, pageInfo *common.PageInfo) ([]AdminAnnouncementSummary, int64, error) {
	query := DB.Model(&Announcement{})
	if app != "" {
		query = query.Where("app = ?", app)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []AdminAnnouncementSummary
	err := query.
		Select("id, app, title, type, publish_time, created_at, enabled, updated_at").
		Order("publish_time DESC, id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&items).Error
	return items, total, err
}

// GetAnnouncementById 管理侧按 id 取（不校验 enabled/publish_time）
func GetAnnouncementById(id int) (*Announcement, error) {
	if id <= 0 {
		return nil, nil
	}
	var a Announcement
	err := DB.First(&a, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
