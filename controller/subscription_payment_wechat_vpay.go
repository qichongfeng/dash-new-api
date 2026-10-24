package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// WeChat mini-program virtual payment (个人主体虚拟支付, 道具直购).
// The mini-program backend (holding a root credential) creates the order for its
// end user against the app's configuration (resolved by AppID), returns payData to
// wx.requestVirtualPayment, and WeChat delivers via the XML push on
// /api/subscription/wechat-vpay/notify/:app_id.

// wechatVpayOrderPayload is stored in SubscriptionOrder.ProviderPayload between order
// creation and completion. After completion the field is overwritten with the raw
// deliver push, so refund/reconcile paths must not rely on it staying JSON.
type wechatVpayOrderPayload struct {
	OpenId     string `json:"openid"`
	ProductId  string `json:"product_id"`
	GoodsPrice int64  `json:"goods_price"`
	Quantity   int    `json:"quantity"`
}

type AdminWechatVpayOrderRequest struct {
	PlanId     int    `json:"plan_id"`
	AppId      string `json:"app_id"`
	OpenId     string `json:"openid"`
	SessionKey string `json:"session_key"`
}

// AdminCreateWechatVpayOrder creates a pending subscription order for the target user
// and returns the signed payData for wx.requestVirtualPayment. app_id selects the
// mini-program the openid/session_key belong to. session_key is used only to compute
// the user-mode signature and is never persisted or logged.
func AdminCreateWechatVpayOrder(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isWechatVpayTopUpEnabled() {
		common.ApiErrorMsg(c, "微信虚拟支付未配置或未启用")
		return
	}
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req AdminWechatVpayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	app, err := resolveWechatVpayAppForOrder(c, req.AppId)
	if err != nil {
		return
	}
	openid := strings.TrimSpace(req.OpenId)
	sessionKey := strings.TrimSpace(req.SessionKey)
	if openid == "" || sessionKey == "" {
		common.ApiErrorMsg(c, "openid 和 session_key 不能为空")
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user.Status != common.UserStatusEnabled {
		common.ApiErrorMsg(c, "用户已被禁用")
		return
	}
	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	productId := plan.WechatVpayProductIdForApp(app.AppId)
	if productId == "" {
		common.ApiErrorMsg(c, "该套餐未配置此小程序的道具ID")
		return
	}
	// 套餐以 USD 计价，按后台"美元汇率"换算为微信道具的人民币价格（单位：分）。
	// 换算结果必须为正整数分，且与 MP 后台道具登记价格完全一致，否则微信侧拉起支付失败。
	goodsPrice, err := service.WechatVpayGoodsPriceFen(plan.PriceAmount, operation_setting.USDExchangeRate)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}
	appKey := app.AppKeyForEnv()
	if appKey == "" {
		common.ApiErrorMsg(c, "该小程序未配置微信虚拟支付 AppKey")
		return
	}

	// outTradeNo: 8-32 位、唯一、不能以下划线开头。
	tradeNo := fmt.Sprintf("WVP%d%s", time.Now().UnixMilli(), common.GetRandomString(9))
	payload, err := common.Marshal(wechatVpayOrderPayload{
		OpenId:     openid,
		ProductId:  productId,
		GoodsPrice: goodsPrice,
		Quantity:   1,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	order := &model.SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           float64(goodsPrice) / 100, // 实际人民币扣款额（元）
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWechatVpay,
		PaymentProvider: model.PaymentProviderWechatVpay,
		ProviderAppId:   app.AppId,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
		ProviderPayload: string(payload),
	}
	if err := order.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	payData, err := service.BuildWechatVpayPayData(appKey, &service.WechatVpaySignData{
		OfferId:      strings.TrimSpace(app.OfferId),
		BuyQuantity:  1,
		Env:          app.Env,
		CurrencyType: "CNY",
		ProductId:    productId,
		GoodsPrice:   goodsPrice,
		OutTradeNo:   tradeNo,
		Attach:       tradeNo,
	}, sessionKey)
	if err != nil {
		order.Status = common.TopUpStatusFailed
		_ = order.Update()
		common.ApiErrorMsg(c, "生成支付签名失败")
		return
	}

	recordManageAuditFor(c, user.Id, "subscription.vpay_order_create", map[string]any{
		"plan_id":  plan.Id,
		"trade_no": tradeNo,
		"app_id":   app.AppId,
	})
	common.ApiSuccess(c, gin.H{
		"order_id": tradeNo,
		"pay_data": payData,
	})
}

// resolveWechatVpayAppForOrder loads the mini-program app named by the order request
// and validates it can sign orders. Writes the error response and returns nil on failure.
func resolveWechatVpayAppForOrder(c *gin.Context, appId string) (*model.WechatVpayApp, error) {
	app, err := model.GetWechatVpayAppByAppId(appId)
	if err != nil {
		common.ApiError(c, err)
		return nil, err
	}
	if app == nil || !app.SigningConfigured() {
		common.ApiErrorMsg(c, "小程序不存在或虚拟支付未配置完整")
		return nil, errors.New("wechat vpay app not usable")
	}
	return app, nil
}

// AdminGetWechatVpayOrder returns the local order state for mini-program backend polling.
func AdminGetWechatVpayOrder(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.PaymentProvider != model.PaymentProviderWechatVpay {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	common.ApiSuccess(c, gin.H{
		"order_id":        order.TradeNo,
		"user_id":         order.UserId,
		"plan_id":         order.PlanId,
		"money":           order.Money,
		"provider_app_id": order.ProviderAppId,
		"status":          order.Status,
		"create_time":     order.CreateTime,
		"complete_time":   order.CompleteTime,
	})
}

// AdminQueryWechatVpayOrderRemote calls WeChat /xpay/query_order as the delivery
// fallback: a paid pending order is completed, a refunded order is reconciled.
func AdminQueryWechatVpayOrderRemote(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.PaymentProvider != model.PaymentProviderWechatVpay {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	openid := wechatVpayOrderOpenId(order)
	if openid == "" {
		common.ApiErrorMsg(c, "订单缺少 openid，无法查单")
		return
	}
	app, err := resolveWechatVpayAppForOrder(c, order.ProviderAppId)
	if err != nil {
		return
	}
	result, raw, err := service.QueryWechatVpayOrder(c.Request.Context(), app, openid, tradeNo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	completed := false
	refundApplied := false
	if result.Order != nil && service.WechatVpayOrderStatusIsPaid(result.Order.Status) &&
		order.Status == common.TopUpStatusPending {
		LockOrder(tradeNo)
		err = model.CompleteSubscriptionOrder(tradeNo, string(raw), model.PaymentProviderWechatVpay, model.PaymentMethodWechatVpay)
		UnlockOrder(tradeNo)
		if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
			common.ApiError(c, err)
			return
		}
		completed = err == nil
	}
	if result.Order != nil &&
		(result.Order.Status == service.WechatVpayOrderStatusRefunded || result.Order.Status == service.WechatVpayOrderStatusUserRefunded) {
		refundApplied = markWechatVpayOrderRefunded(order, result.Order.RefundFee)
	}
	common.ApiSuccess(c, gin.H{
		"remote":         result,
		"completed":      completed,
		"refund_applied": refundApplied,
	})
}

// WechatVpayNotify receives the WeChat MP message-push channel, routed per
// mini-program by the :app_id path segment (each MP console configures its own URL).
// GET is the MP console URL verification; POST carries the XML event pushes.
func WechatVpayNotify(c *gin.Context) {
	appId := strings.TrimSpace(c.Param("app_id"))
	app, err := model.GetWechatVpayAppByAppId(appId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if app == nil || !app.PushConfigured() {
		// 未知/未启用小程序的推送一律拒绝（含 URL 验证）
		c.String(http.StatusForbidden, "webhook disabled")
		return
	}
	if c.Request.Method == http.MethodGet {
		echostr := c.Query("echostr")
		if service.VerifyWechatVpayPushSignature(app.PushToken, c.Query("timestamp"), c.Query("nonce"), echostr, c.Query("signature")) {
			c.String(http.StatusOK, echostr)
		} else {
			c.String(http.StatusForbidden, "verify failed")
		}
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "bad request")
		return
	}
	// 明文模式：被签名的加密段为空串。
	if !service.VerifyWechatVpayPushSignature(app.PushToken, c.Query("timestamp"), c.Query("nonce"), "", c.Query("signature")) {
		common.SysError("微信虚拟支付推送验签失败: app=" + app.AppId)
		c.String(http.StatusUnauthorized, "invalid signature")
		return
	}
	msg, err := service.ParseWechatVpayPush(body)
	if err != nil {
		common.SysError("微信虚拟支付推送解析失败: " + err.Error())
		// 重试无益，直接确认。
		c.String(http.StatusOK, "success")
		return
	}
	if msg.Env != app.Env {
		common.SysError(fmt.Sprintf("微信虚拟支付推送 env 不匹配: app=%s push=%d configured=%d", app.AppId, msg.Env, app.Env))
		c.String(http.StatusOK, "success")
		return
	}
	switch msg.Event {
	case service.WechatVpayEventGoodsDeliver:
		handleWechatVpayDeliverPush(c, app, msg, body)
	case service.WechatVpayEventRefund:
		handleWechatVpayRefundPush(c, app, msg)
	default:
		common.SysLog("微信虚拟支付推送未处理事件: " + msg.Event)
		c.String(http.StatusOK, "success")
	}
}

// wechatVpayOrderBelongsToApp guards against a push of one mini-program completing
// another app's order (trade_no is globally unique, so a mismatch is permanent —
// ack success to stop retries).
func wechatVpayOrderBelongsToApp(order *model.SubscriptionOrder, app *model.WechatVpayApp) bool {
	if order.PaymentProvider != model.PaymentProviderWechatVpay {
		return false
	}
	if order.ProviderAppId == "" || order.ProviderAppId == app.AppId {
		return true
	}
	common.SysError(fmt.Sprintf("微信虚拟支付订单 %s 归属小程序 %s 与推送小程序 %s 不符", order.TradeNo, order.ProviderAppId, app.AppId))
	return false
}

func handleWechatVpayDeliverPush(c *gin.Context, app *model.WechatVpayApp, msg *service.WechatVpayPushMessage, body []byte) {
	tradeNo := strings.TrimSpace(msg.OutTradeNo)
	if tradeNo == "" {
		common.SysError("微信虚拟支付发货推送缺少 OutTradeNo")
		c.String(http.StatusOK, "success")
		return
	}
	if order := model.GetSubscriptionOrderByTradeNo(tradeNo); order != nil && !wechatVpayOrderBelongsToApp(order, app) {
		c.String(http.StatusOK, "success")
		return
	}
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	err := model.CompleteSubscriptionOrder(tradeNo, string(body), model.PaymentProviderWechatVpay, model.PaymentMethodWechatVpay)
	switch {
	case err == nil:
		c.String(http.StatusOK, "success")
	case errors.Is(err, model.ErrSubscriptionOrderNotFound), errors.Is(err, model.ErrPaymentMethodMismatch):
		// 永久性失败：确认收到，停止重试（同时避免恶意探测触发重试风暴）。
		common.SysError(fmt.Sprintf("微信虚拟支付发货推送无法匹配订单 %s: %v", tradeNo, err))
		c.String(http.StatusOK, "success")
	default:
		// 瞬时错误：返回失败让平台重试（最多 15 次）。
		common.SysError(fmt.Sprintf("微信虚拟支付发货推送处理失败 %s: %v", tradeNo, err))
		c.String(http.StatusInternalServerError, "retry")
	}
}

func handleWechatVpayRefundPush(c *gin.Context, app *model.WechatVpayApp, msg *service.WechatVpayPushMessage) {
	tradeNo := strings.TrimSpace(msg.MchOrderId)
	if tradeNo == "" {
		common.SysError("微信虚拟支付退款推送缺少 MchOrderId")
		c.String(http.StatusOK, "success")
		return
	}
	if msg.RetCode != 0 {
		common.SysLog(fmt.Sprintf("微信虚拟支付退款未成功 %s: ret_code=%d ret_msg=%s", tradeNo, msg.RetCode, msg.RetMsg))
		c.String(http.StatusOK, "success")
		return
	}
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.PaymentProvider != model.PaymentProviderWechatVpay {
		common.SysError("微信虚拟支付退款推送无法匹配订单: " + tradeNo)
		c.String(http.StatusOK, "success")
		return
	}
	if !wechatVpayOrderBelongsToApp(order, app) {
		c.String(http.StatusOK, "success")
		return
	}
	markWechatVpayOrderRefunded(order, msg.RefundFee)
	c.String(http.StatusOK, "success")
}

// markWechatVpayOrderRefunded marks the order refunded and cancels the still-active
// subscription that the order created. Idempotent: an already-refunded order is a no-op.
func markWechatVpayOrderRefunded(order *model.SubscriptionOrder, refundFee int64) bool {
	if order.Status == common.TopUpStatusRefunded {
		return false
	}
	order.Status = common.TopUpStatusRefunded
	if err := order.Update(); err != nil {
		common.SysError(fmt.Sprintf("微信虚拟支付订单标记退款失败 %s: %v", order.TradeNo, err))
		return false
	}
	cancelledId, err := model.CancelUserSubscriptionByTradeNo(order.TradeNo)
	if err != nil {
		common.SysError(fmt.Sprintf("微信虚拟支付退款作废订阅失败 %s: %v", order.TradeNo, err))
	}
	if cancelledId > 0 {
		model.RecordLog(order.UserId, model.LogTypeTopup, fmt.Sprintf("订阅订单已退款（%d 分），订阅已作废", refundFee))
	} else {
		model.RecordLog(order.UserId, model.LogTypeTopup, fmt.Sprintf("订阅订单已退款（%d 分），未找到需作废的有效订阅", refundFee))
	}
	common.SysLog(fmt.Sprintf("微信虚拟支付订单 %s 已标记退款（%d 分），作废订阅 %d", order.TradeNo, refundFee, cancelledId))
	return true
}

// wechatVpayOrderOpenId extracts the buyer openid from the order payload: JSON before
// completion, the deliver push XML afterwards.
func wechatVpayOrderOpenId(order *model.SubscriptionOrder) string {
	var payload wechatVpayOrderPayload
	if err := common.Unmarshal([]byte(order.ProviderPayload), &payload); err == nil && payload.OpenId != "" {
		return payload.OpenId
	}
	if msg, err := service.ParseWechatVpayPush([]byte(order.ProviderPayload)); err == nil {
		return strings.TrimSpace(msg.OpenId)
	}
	return ""
}
