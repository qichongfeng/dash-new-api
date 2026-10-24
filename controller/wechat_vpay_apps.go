package controller

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// 微信小程序虚拟支付多小程序管理：每个小程序一行配置（AppID/OfferID/AppKey/推送
// Token 等均为 appid 维度）。凭证只写不读：列表/详情经视图层脱敏，密钥留空 = 保留原值。

type AdminWechatVpayAppRequest struct {
	AppId         string `json:"app_id"`
	Name          string `json:"name"`
	AppSecret     string `json:"app_secret"`
	OfferId       string `json:"offer_id"`
	AppKey        string `json:"app_key"`
	SandboxAppKey string `json:"sandbox_app_key"`
	PushToken     string `json:"push_token"`
	Env           int    `json:"env"`
	Enabled       *bool  `json:"enabled"`
}

// AdminWechatVpayAppView 管理面视图：不含任何凭证明文，只带 has_* 配置状态。
type AdminWechatVpayAppView struct {
	Id               int    `json:"id"`
	AppId            string `json:"app_id"`
	Name             string `json:"name"`
	OfferId          string `json:"offer_id"`
	Env              int    `json:"env"`
	Enabled          bool   `json:"enabled"`
	HasAppSecret     bool   `json:"has_app_secret"`
	HasAppKey        bool   `json:"has_app_key"`
	HasSandboxAppKey bool   `json:"has_sandbox_app_key"`
	HasPushToken     bool   `json:"has_push_token"`
	SigningReady     bool   `json:"signing_ready"`
	PushReady        bool   `json:"push_ready"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func wechatVpayAppView(app *model.WechatVpayApp) AdminWechatVpayAppView {
	return AdminWechatVpayAppView{
		Id:               app.Id,
		AppId:            app.AppId,
		Name:             app.Name,
		OfferId:          app.OfferId,
		Env:              app.Env,
		Enabled:          app.Enabled,
		HasAppSecret:     strings.TrimSpace(app.AppSecret) != "",
		HasAppKey:        strings.TrimSpace(app.AppKey) != "",
		HasSandboxAppKey: strings.TrimSpace(app.SandboxAppKey) != "",
		HasPushToken:     strings.TrimSpace(app.PushToken) != "",
		SigningReady:     app.SigningConfigured(),
		PushReady:        app.PushConfigured(),
		CreatedAt:        app.CreatedAt,
		UpdatedAt:        app.UpdatedAt,
	}
}

// validateWechatVpayAppInput 校验请求中的非密钥字段，返回规范化后的值。
func validateWechatVpayAppInput(appId, name, offerId string, env int) (string, string, string, error) {
	appId = strings.TrimSpace(appId)
	if appId == "" || len(appId) > 32 {
		return "", "", "", errors.New("AppID 不能为空且不超过 32 字符")
	}
	for _, r := range appId {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return "", "", "", errors.New("AppID 只能包含字母、数字、-、_")
		}
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 64 {
		return "", "", "", errors.New("名称不能超过 64 字符")
	}
	if env != 0 && env != 1 {
		return "", "", "", errors.New("Env 只能为 0（现网）或 1（沙箱）")
	}
	return appId, name, strings.TrimSpace(offerId), nil
}

func AdminListWechatVpayApps(c *gin.Context) {
	apps, err := model.GetWechatVpayApps()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	views := make([]AdminWechatVpayAppView, 0, len(apps))
	for _, app := range apps {
		views = append(views, wechatVpayAppView(app))
	}
	common.ApiSuccess(c, views)
}

func AdminGetWechatVpayApp(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	app, err := model.GetWechatVpayAppById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app == nil {
		common.ApiErrorMsg(c, "小程序不存在")
		return
	}
	common.ApiSuccess(c, wechatVpayAppView(app))
}

func AdminCreateWechatVpayApp(c *gin.Context) {
	var req AdminWechatVpayAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	appId, name, offerId, err := validateWechatVpayAppInput(req.AppId, req.Name, req.OfferId, req.Env)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	existing, err := model.GetWechatVpayAppByAppId(appId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing != nil {
		common.ApiErrorMsg(c, "该 AppID 已存在")
		return
	}
	app := &model.WechatVpayApp{
		AppId:         appId,
		Name:          name,
		AppSecret:     strings.TrimSpace(req.AppSecret),
		OfferId:       offerId,
		AppKey:        strings.TrimSpace(req.AppKey),
		SandboxAppKey: strings.TrimSpace(req.SandboxAppKey),
		PushToken:     strings.TrimSpace(req.PushToken),
		Env:           req.Env,
		Enabled:       true,
	}
	if req.Enabled != nil {
		app.Enabled = *req.Enabled
	}
	if err := model.CreateWechatVpayApp(app); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "wechat_vpay.app_create", map[string]any{
		"app_id": app.AppId,
		"name":   app.Name,
	})
	common.ApiSuccess(c, wechatVpayAppView(app))
}

func AdminUpdateWechatVpayApp(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminWechatVpayAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	app, err := model.GetWechatVpayAppById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app == nil {
		common.ApiErrorMsg(c, "小程序不存在")
		return
	}
	appId, name, offerId, err := validateWechatVpayAppInput(req.AppId, req.Name, req.OfferId, req.Env)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if appId != app.AppId {
		other, err := model.GetWechatVpayAppByAppId(appId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if other != nil && other.Id != app.Id {
			common.ApiErrorMsg(c, "该 AppID 已存在")
			return
		}
		// AppID 变更使存量订单的 provider_app_id 失配，禁止；删除后重建可改。
		var orders int64
		if err := model.DB.Model(&model.SubscriptionOrder{}).
			Where("payment_provider = ? AND provider_app_id = ?",
				model.PaymentProviderWechatVpay, app.AppId).
			Count(&orders).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		if orders > 0 {
			common.ApiErrorMsg(c, "该小程序已有订单，不能修改 AppID（如需更换请删除后重建）")
			return
		}
		app.AppId = appId
	}
	app.Name = name
	app.OfferId = offerId
	app.Env = req.Env
	if req.Enabled != nil {
		app.Enabled = *req.Enabled
	}
	// 密钥类字段：留空 = 保留原值
	if v := strings.TrimSpace(req.AppSecret); v != "" {
		app.AppSecret = v
	}
	if v := strings.TrimSpace(req.AppKey); v != "" {
		app.AppKey = v
	}
	if v := strings.TrimSpace(req.SandboxAppKey); v != "" {
		app.SandboxAppKey = v
	}
	if v := strings.TrimSpace(req.PushToken); v != "" {
		app.PushToken = v
	}
	if err := model.UpdateWechatVpayApp(app); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "wechat_vpay.app_update", map[string]any{
		"app_id": app.AppId,
		"name":   app.Name,
	})
	common.ApiSuccess(c, wechatVpayAppView(app))
}

func AdminDeleteWechatVpayApp(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	app, err := model.GetWechatVpayAppById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app == nil {
		common.ApiErrorMsg(c, "小程序不存在")
		return
	}
	err = model.DeleteWechatVpayAppById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, model.ErrWechatVpayAppNotFound) {
			common.ApiErrorMsg(c, "小程序不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "wechat_vpay.app_delete", map[string]any{
		"app_id": app.AppId,
		"name":   app.Name,
	})
	common.ApiSuccess(c, nil)
}

type AdminWechatVpayGoodsProbeRequest struct {
	// 道具图片（jpg/png，微信侧拉取校验）；留空回退 <ServerAddress>/logo.png
	ItemUrl string `json:"item_url"`
}

// wechatVpayGoodsProbeVerdict maps the probe errcode to an actionable hint.
// xpay business errors (268490xxx) are returned by the xpay service itself,
// which implies the call passed the auth layer — i.e. the app may call the
// goods APIs. The uploaded item is a dev-version item named 接口探测道具 and
// can be deleted in the MP console.
func wechatVpayGoodsProbeVerdict(errcode int) string {
	switch errcode {
	case 0:
		return "接口可用：测试道具已上传到开发版本（MP 后台道具管理可见，可删）。可以做「套餐自动推送道具」链路。"
	case 268490002:
		return "请求已被 xpay 接收但参数被拒（大概率已通过鉴权）：常见原因是 item_url 不可公网访问或格式不对，换一个可访问的 jpg/png 地址重试。"
	case 268490003:
		return "签名错误：检查该小程序的 AppKey 是否与 Env（现网/沙箱）匹配。"
	case 268490007:
		return "内容安全拦截（图片/文案敏感）：接口权限存在，换图片或改名称重试。"
	case 268490012:
		return "有批量任务运行中：接口权限存在，稍等后重试。"
	case 268490015:
		return "频率限制：接口权限存在，稍后重试。"
	case 40001, 40014, 42001:
		return "access_token 无效或过期：检查该小程序的 AppSecret 是否正确。"
	case 48001:
		return "api unauthorized：该小程序无权调用道具接口（个人主体很可能不支持 API 上传，需在 MP 后台手动建道具）。"
	default:
		return "未识别的返回码，参考 errmsg 与官方错误码文档判断。"
	}
}

// AdminWechatVpayAppGoodsProbe uploads one throwaway test item via
// /xpay/start_upload_goods to find out whether this mini-program (个人主体 doc
// does not list the goods APIs) may call the goods APIs at all — the
// precondition for automating item sync from plans. Dev-version only, never
// published, deletable in the MP console.
func AdminWechatVpayAppGoodsProbe(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminWechatVpayGoodsProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	app, err := model.GetWechatVpayAppById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app == nil {
		common.ApiErrorMsg(c, "小程序不存在")
		return
	}
	if strings.TrimSpace(app.AppSecret) == "" {
		common.ApiErrorMsg(c, "探测需要该小程序配置 AppSecret（换取 access_token）")
		return
	}
	itemUrl := strings.TrimSpace(req.ItemUrl)
	if itemUrl == "" {
		addr := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
		if addr == "" {
			common.ApiErrorMsg(c, "未配置服务器地址，请显式传入 item_url（公网可访问的 jpg/png）")
			return
		}
		itemUrl = addr + "/logo.png"
	}
	item := service.WechatVpayUploadItem{
		Id:      "probe_test_item",
		Name:    "接口探测道具",
		Price:   1,
		Remark:  "dash-new-api 道具接口权限探测，可删除",
		ItemUrl: itemUrl,
	}
	result, raw, err := service.StartWechatVpayUploadGoods(c.Request.Context(), app, item)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "wechat_vpay.goods_probe", map[string]any{
		"app_id":  app.AppId,
		"errcode": result.ErrCode,
		"errmsg":  result.ErrMsg,
	})
	common.ApiSuccess(c, gin.H{
		"errcode": result.ErrCode,
		"errmsg":  result.ErrMsg,
		"verdict": wechatVpayGoodsProbeVerdict(result.ErrCode),
		"raw":     strings.TrimSpace(string(raw)),
		"item":    item,
	})
}
