package model

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// WechatVpayApp 微信小程序虚拟支付配置，每个接入的小程序一行。AppID 是自然键：
// openid/session_key、道具登记（productId 与价格）、消息推送 URL+Token、access_token
// 全是 appid 维度，多小程序并存时各自独立配置、互不影响。
type WechatVpayApp struct {
	Id int `json:"id"`

	// 微信小程序 AppID（如 wx1234567890abcdef），推送 URL 以它路由
	AppId string `json:"app_id" gorm:"type:varchar(32);not null;uniqueIndex"`
	// 展示名（管理面辨识用）
	Name string `json:"name" gorm:"type:varchar(64);not null;default:''"`

	// 以下凭证均为该小程序自己的（MP 后台 虚拟支付 → 基本配置 / 消息推送）。
	// AppSecret 仅用于 query_order 兜底换 access_token，可留空；其余字段写出后
	// 不回显（json:"-"），管理接口经视图层返回 has_* 布尔。
	AppSecret     string `json:"-" gorm:"type:varchar(128);default:''"`
	OfferId       string `json:"offer_id" gorm:"type:varchar(64);default:''"`
	AppKey        string `json:"-" gorm:"type:varchar(128);default:''"` // 现网 AppKey
	SandboxAppKey string `json:"-" gorm:"type:varchar(128);default:''"` // 仅 Env=1 时使用
	PushToken     string `json:"-" gorm:"type:varchar(128);default:''"` // 消息推送 Token

	// 0 = 现网（个人主体固定 0），1 = 沙箱
	Env int `json:"env" gorm:"type:int;not null;default:0"`

	// Enabled 由 controller 显式赋值（bool default 标签在 MySQL/PG 上会反复 ALTER）
	Enabled bool `json:"enabled"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (WechatVpayApp) TableName() string {
	return "wechat_vpay_apps"
}

func (a *WechatVpayApp) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

func (a *WechatVpayApp) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = common.GetTimestamp()
	return nil
}

// AppKeyForEnv 返回与 Env 匹配的 AppKey（沙箱=1 用 SandboxAppKey）。
func (a *WechatVpayApp) AppKeyForEnv() string {
	if a.Env == 1 {
		return strings.TrimSpace(a.SandboxAppKey)
	}
	return strings.TrimSpace(a.AppKey)
}

// SigningConfigured 该小程序能否签名下单/查单：启用且 AppId/OfferId/AppKey 三要素齐备。
func (a *WechatVpayApp) SigningConfigured() bool {
	if !a.Enabled {
		return false
	}
	return strings.TrimSpace(a.AppId) != "" &&
		strings.TrimSpace(a.OfferId) != "" &&
		a.AppKeyForEnv() != ""
}

// PushConfigured 该小程序的推送端点是否可用（签名三要素 + 消息推送 Token）。
func (a *WechatVpayApp) PushConfigured() bool {
	return a.SigningConfigured() && strings.TrimSpace(a.PushToken) != ""
}

var ErrWechatVpayAppNotFound = errors.New("wechat vpay app not found")

func GetWechatVpayApps() ([]*WechatVpayApp, error) {
	var apps []*WechatVpayApp
	err := DB.Order("id ASC").Find(&apps).Error
	return apps, err
}

// GetWechatVpayAppById 管理侧按 id 取，不存在返回 (nil, nil)。
func GetWechatVpayAppById(id int) (*WechatVpayApp, error) {
	if id <= 0 {
		return nil, nil
	}
	var app WechatVpayApp
	err := DB.First(&app, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetWechatVpayAppByAppId 按 AppID 取（推送路由、下单解析）。
func GetWechatVpayAppByAppId(appId string) (*WechatVpayApp, error) {
	appId = strings.TrimSpace(appId)
	if appId == "" {
		return nil, nil
	}
	var app WechatVpayApp
	err := DB.First(&app, "app_id = ?", appId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// HasSigningWechatVpayApp 是否存在可签名下单的小程序（支付可用性判断）。查询失败
// 视为不可用（fail closed）。
func HasSigningWechatVpayApp() bool {
	var apps []*WechatVpayApp
	if err := DB.Where("enabled = ?", true).Find(&apps).Error; err != nil {
		return false
	}
	for _, app := range apps {
		if app.SigningConfigured() {
			return true
		}
	}
	return false
}

func CreateWechatVpayApp(app *WechatVpayApp) error {
	return DB.Create(app).Error
}

func UpdateWechatVpayApp(app *WechatVpayApp) error {
	return DB.Save(app).Error
}

// DeleteWechatVpayAppById 删除小程序配置；仍有待支付订单时拒绝（避免推送/查单失去归属）。
func DeleteWechatVpayAppById(id int) error {
	app, err := GetWechatVpayAppById(id)
	if err != nil {
		return err
	}
	if app == nil {
		return ErrWechatVpayAppNotFound
	}
	var pending int64
	if err := DB.Model(&SubscriptionOrder{}).
		Where("payment_provider = ? AND provider_app_id = ? AND status = ?",
			PaymentProviderWechatVpay, app.AppId, common.TopUpStatusPending).
		Count(&pending).Error; err != nil {
		return err
	}
	if pending > 0 {
		return errors.New("该小程序仍有待支付订单，无法删除")
	}
	return DB.Delete(&WechatVpayApp{}, id).Error
}

// wechatVpayLegacyOptionKeys 单小程序时代的全局 option，迁移后删除。
var wechatVpayLegacyOptionKeys = []string{
	"WechatVpayAppId",
	"WechatVpayAppSecret",
	"WechatVpayOfferId",
	"WechatVpayAppKey",
	"WechatVpaySandboxAppKey",
	"WechatVpayPushToken",
	"WechatVpayEnv",
}

// migrateWechatVpayLegacyOptions 把单小程序时代的 7 个全局 option 一次性迁移为
// wechat_vpay_apps 一行：订单回填 provider_app_id、套餐旧单值道具 ID 回填为映射 JSON。
// 幂等：目标 app 行已存在或旧 key 已清理时仅做清理动作。
func migrateWechatVpayLegacyOptions() error {
	var rows []Option
	if err := DB.Where(commonKeyCol+" IN ?", wechatVpayLegacyOptionKeys).Find(&rows).Error; err != nil {
		return err
	}
	opts := make(map[string]string, len(rows))
	for _, row := range rows {
		opts[row.Key] = row.Value
	}
	appId := strings.TrimSpace(opts["WechatVpayAppId"])
	if appId != "" {
		existing, err := GetWechatVpayAppByAppId(appId)
		if err != nil {
			return err
		}
		if existing == nil {
			env, _ := strconv.Atoi(strings.TrimSpace(opts["WechatVpayEnv"]))
			app := &WechatVpayApp{
				AppId:         appId,
				Name:          appId,
				AppSecret:     strings.TrimSpace(opts["WechatVpayAppSecret"]),
				OfferId:       strings.TrimSpace(opts["WechatVpayOfferId"]),
				AppKey:        strings.TrimSpace(opts["WechatVpayAppKey"]),
				SandboxAppKey: strings.TrimSpace(opts["WechatVpaySandboxAppKey"]),
				PushToken:     strings.TrimSpace(opts["WechatVpayPushToken"]),
				Env:           max(env, 0),
				Enabled:       true,
			}
			if err := CreateWechatVpayApp(app); err != nil {
				return err
			}
			common.SysLog("微信虚拟支付：旧全局配置已迁移为小程序 " + appId)
		}
		// 存量订单归属迁移出的小程序；provider_app_id 为空串（新订单都显式写入）
		if err := DB.Model(&SubscriptionOrder{}).
			Where("payment_provider = ? AND (provider_app_id = '' OR provider_app_id IS NULL)",
				PaymentProviderWechatVpay).
			Update("provider_app_id", appId).Error; err != nil {
			return err
		}
		if err := migrateWechatVpayPlanProductIds(appId); err != nil {
			return err
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return DB.Where(commonKeyCol+" IN ?", wechatVpayLegacyOptionKeys).Delete(&Option{}).Error
}

// migrateWechatVpayPlanProductIds 把套餐的旧单值道具 ID 列迁为 appid → 道具 ID 的
// JSON 映射。旧列在历史库才存在（新库不再创建）；映射已写过的行不动。
func migrateWechatVpayPlanProductIds(appId string) error {
	hasLegacyCol := false
	cols, err := DB.Migrator().ColumnTypes(&SubscriptionPlan{})
	if err != nil {
		return err
	}
	for _, col := range cols {
		if col.Name() == "wechat_vpay_product_id" {
			hasLegacyCol = true
			break
		}
	}
	if !hasLegacyCol {
		return nil
	}
	var plans []struct {
		Id                     int    `gorm:"column:id"`
		WechatVpayProductIdCol string `gorm:"column:wechat_vpay_product_id"`
	}
	if err := DB.Table("subscription_plans").
		Select("id, wechat_vpay_product_id").
		Where("wechat_vpay_product_id <> '' AND (wechat_vpay_product_ids = '' OR wechat_vpay_product_ids IS NULL)").
		Find(&plans).Error; err != nil {
		return err
	}
	for _, plan := range plans {
		payload, err := common.Marshal(map[string]string{appId: strings.TrimSpace(plan.WechatVpayProductIdCol)})
		if err != nil {
			return err
		}
		if err := DB.Table("subscription_plans").
			Where("id = ?", plan.Id).
			Update("wechat_vpay_product_ids", string(payload)).Error; err != nil {
			return err
		}
	}
	if len(plans) > 0 {
		common.SysLog("微信虚拟支付：套餐道具 ID 已迁移为小程序映射")
	}
	return nil
}
