package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// WeChat mini-program virtual payment (个人主体虚拟支付, 道具直购 mode).
// Docs: https://developers.weixin.qq.com/miniprogram/dev/platform-capabilities/business-capabilities/virtual-payment.html
// and .../virtual-payment/person (personal subject).

const (
	// WechatVpayEventGoodsDeliver is pushed after a cash goods order is paid.
	WechatVpayEventGoodsDeliver = "xpay_goods_deliver_notify"
	// WechatVpayEventRefund is pushed after a refund completes (RetCode 0 = success).
	WechatVpayEventRefund = "xpay_refund_notify"

	// WechatVpayPayMode is the fixed wx.requestVirtualPayment mode for 道具直购.
	WechatVpayPayMode = "short_series_goods"

	// wechatVpayClientUri is the literal uri used in the pay signature for
	// wx.requestVirtualPayment (no leading slash, per official spec).
	wechatVpayClientUri = "requestVirtualPayment"

	wechatVpayQueryOrderUri  = "/xpay/query_order"
	wechatVpayUploadGoodsUri = "/xpay/start_upload_goods"
	wechatVpayTokenUrl       = "https://api.weixin.qq.com/cgi-bin/token"
	wechatVpayApiBase        = "https://api.weixin.qq.com"
)

// query_order order.status values (official spec).
const (
	WechatVpayOrderStatusInit           = 0  // 订单初始化（不可支付）
	WechatVpayOrderStatusCreated        = 1  // 订单创建成功（未支付）
	WechatVpayOrderStatusPaid           = 2  // 已支付，待发货
	WechatVpayOrderStatusDelivering     = 3  // 发货中
	WechatVpayOrderStatusDelivered      = 4  // 已发货
	WechatVpayOrderStatusRefunded       = 5  // 已退款
	WechatVpayOrderStatusClosed         = 6  // 已关闭
	WechatVpayOrderStatusRefundFailed   = 7  // 退款失败
	WechatVpayOrderStatusUserRefunded   = 8  // 用户退款完成
	WechatVpayOrderStatusAdverRecovered = 9  // 回收广告金完成
	WechatVpayOrderStatusSettleBack     = 10 // 分账回退完成
)

// WechatVpayOrderStatusIsPaid reports whether a queried order was actually paid
// (the states from which delivery/completion is legitimate).
func WechatVpayOrderStatusIsPaid(status int) bool {
	return status == WechatVpayOrderStatusPaid ||
		status == WechatVpayOrderStatusDelivering ||
		status == WechatVpayOrderStatusDelivered
}

// WechatVpayCalcPaySig computes the pay signature: hex(HMAC-SHA256(appKey, uri + "&" + signData)).
// For wx.requestVirtualPayment the uri is the literal "requestVirtualPayment"; for server
// APIs it is the request path such as "/xpay/query_order".
// Official vector: uri=/xpay/query_user_balance,
// signData={"openid": "xxx", "user_ip": "127.0.0.1", "env": 0}, appKey=12345
// -> c37809f27c6d7fd1837ad2500a04512b66b34fd793a39a385fade56dca89a4b5
func WechatVpayCalcPaySig(appKey, uri, signData string) string {
	mac := hmac.New(sha256.New, []byte(appKey))
	mac.Write([]byte(uri + "&" + signData))
	return hex.EncodeToString(mac.Sum(nil))
}

// WechatVpayCalcUserSig computes the user-mode signature over signData with the
// mini-program session key: hex(HMAC-SHA256(sessionKey, signData)).
// Official vector: signData={"openid": "xxx", "user_ip": "127.0.0.1", "env": 0},
// sessionKey=9hAb/NEYUlkaMBEsmFgzig==
// -> 089d9e8dc5d308977360c4b79ec600a93d736802802a807d634192328032f6c7
func WechatVpayCalcUserSig(sessionKey, signData string) string {
	mac := hmac.New(sha256.New, []byte(sessionKey))
	mac.Write([]byte(signData))
	return hex.EncodeToString(mac.Sum(nil))
}

// WechatVpaySignData is the JSON payload signed into payData for
// wx.requestVirtualPayment. The marshaled string is returned to the mini-program
// verbatim, so field order is irrelevant as long as we sign what we return.
type WechatVpaySignData struct {
	OfferId      string `json:"offerId"`
	BuyQuantity  int    `json:"buyQuantity"`
	Env          int    `json:"env"`
	CurrencyType string `json:"currencyType"`
	ProductId    string `json:"productId"`
	GoodsPrice   int64  `json:"goodsPrice"` // 单位：分，须与 MP 后台道具价格一致
	OutTradeNo   string `json:"outTradeNo"`
	Attach       string `json:"attach"`
}

// WechatVpayPayData is what the mini-program passes to wx.requestVirtualPayment.
type WechatVpayPayData struct {
	Mode      string `json:"mode"`
	SignData  string `json:"signData"`
	PaySig    string `json:"paySig"`
	Signature string `json:"signature"`
}

// WechatVpayGoodsPriceFen converts a USD plan price into the WeChat item price
// in fen (1/100 CNY) via the admin-configured USD→CNY exchange rate. WeChat
// requires a whole-fen price that exactly matches the item registered in the
// MP console, so a conversion landing between fen is rejected.
func WechatVpayGoodsPriceFen(priceAmount, usdExchangeRate float64) (int64, error) {
	if priceAmount <= 0 || usdExchangeRate <= 0 {
		return 0, fmt.Errorf("套餐价格必须为正数")
	}
	priceInFen := priceAmount * usdExchangeRate * 100
	goodsPrice := int64(math.Round(priceInFen))
	if math.Abs(priceInFen-float64(goodsPrice)) > 1e-4 {
		return 0, fmt.Errorf("套餐价格×美元汇率换算后必须精确到分，请调整套餐价格或汇率")
	}
	return goodsPrice, nil
}

// BuildWechatVpayPayData signs a 道具直购 order for wx.requestVirtualPayment.
// appKey must match env (use WechatVpayAppKeyForEnv).
func BuildWechatVpayPayData(appKey string, sign *WechatVpaySignData, sessionKey string) (*WechatVpayPayData, error) {
	if sign == nil {
		return nil, fmt.Errorf("sign data is nil")
	}
	signData, err := common.Marshal(sign)
	if err != nil {
		return nil, err
	}
	signDataStr := string(signData)
	return &WechatVpayPayData{
		Mode:      WechatVpayPayMode,
		SignData:  signDataStr,
		PaySig:    WechatVpayCalcPaySig(appKey, wechatVpayClientUri, signDataStr),
		Signature: WechatVpayCalcUserSig(sessionKey, signDataStr),
	}, nil
}

// WechatVpayPushWeChatPayInfo mirrors WeChatPayInfo in the deliver push.
type WechatVpayPushWeChatPayInfo struct {
	MchOrderNo    string `xml:"MchOrderNo"` // 平台单号 wx_order_id（幂等以此为准）
	TransactionId string `xml:"TransactionId"`
	PaidTime      int64  `xml:"PaidTime"`
}

// WechatVpayPushGoodsInfo mirrors GoodsInfo in the deliver push.
type WechatVpayPushGoodsInfo struct {
	ProductId   string `xml:"ProductId"`
	Quantity    int    `xml:"Quantity"`
	OrigPrice   int64  `xml:"OrigPrice"`
	ActualPrice int64  `xml:"ActualPrice"`
	Attach      string `xml:"Attach"`
}

// WechatVpayPushMessage is the XML message pushed on the MP 消息推送 channel.
// Fields beyond Event are populated per event type; refund-only fields stay zero
// for deliver pushes and vice versa.
type WechatVpayPushMessage struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Event        string `xml:"Event"`

	// deliver push
	OpenId        string                       `xml:"OpenId"`
	OutTradeNo    string                       `xml:"OutTradeNo"`
	Env           int                          `xml:"Env"`
	WeChatPayInfo *WechatVpayPushWeChatPayInfo `xml:"WeChatPayInfo"`
	GoodsInfo     *WechatVpayPushGoodsInfo     `xml:"GoodsInfo"`

	// refund push
	WxRefundId          string `xml:"WxRefundId"`
	MchRefundId         string `xml:"MchRefundId"`
	WxOrderId           string `xml:"WxOrderId"`
	MchOrderId          string `xml:"MchOrderId"` // = 下单时的 outTradeNo
	RefundFee           int64  `xml:"RefundFee"`
	RetCode             int    `xml:"RetCode"` // 0 = 退款成功
	RetMsg              string `xml:"RetMsg"`
	RefundSuccTimestamp int64  `xml:"RefundSuccTimestamp"`
}

// ParseWechatVpayPush parses the pushed XML body.
func ParseWechatVpayPush(body []byte) (*WechatVpayPushMessage, error) {
	var msg WechatVpayPushMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// VerifyWechatVpayPushSignature verifies the standard WeChat message-push signature:
// sha1 of the sorted concatenation of token, timestamp, nonce and the encrypted
// payload element. In plaintext mode the payload is the empty string; during the
// MP console URL verification it is the echostr. Comparison is constant-time.
func VerifyWechatVpayPushSignature(token, timestamp, nonce, payload, signature string) bool {
	if token == "" || signature == "" {
		return false
	}
	items := []string{token, timestamp, nonce, payload}
	sort.Strings(items)
	sum := sha1.Sum([]byte(strings.Join(items, "")))
	expected := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// ---- access token + query_order ----

// Per-appid access_token cache. One appid has one live token globally; entries are
// keyed by appid so multiple mini-programs refresh independently.
type wechatVpayTokenEntry struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

var wechatVpayAccessTokens sync.Map // appid -> *wechatVpayTokenEntry

func getWechatVpayAccessToken(ctx context.Context, app *model.WechatVpayApp) (string, error) {
	entryAny, _ := wechatVpayAccessTokens.LoadOrStore(app.AppId, &wechatVpayTokenEntry{})
	entry := entryAny.(*wechatVpayTokenEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.token != "" && time.Now().Before(entry.expiresAt) {
		return entry.token, nil
	}
	appSecret := strings.TrimSpace(app.AppSecret)
	if appSecret == "" {
		return "", fmt.Errorf("小程序 %s 未配置 AppSecret", app.AppId)
	}
	reqUrl := fmt.Sprintf("%s?grant_type=client_credential&appid=%s&secret=%s",
		wechatVpayTokenUrl, url.QueryEscape(app.AppId), url.QueryEscape(appSecret))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := common.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析 access_token 响应失败: %w", err)
	}
	if tokenResp.ErrCode != 0 || tokenResp.AccessToken == "" {
		return "", fmt.Errorf("获取 access_token 失败: errcode=%d errmsg=%s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}
	entry.token = tokenResp.AccessToken
	// Refresh 5 minutes before expiry.
	entry.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second - 5*time.Minute)
	return entry.token, nil
}

// WechatVpayQueriedOrder mirrors the order object returned by /xpay/query_order.
type WechatVpayQueriedOrder struct {
	OrderId     string `json:"order_id"`
	Status      int    `json:"status"`
	OrderFee    int64  `json:"order_fee"`
	PaidFee     int64  `json:"paid_fee"`
	RefundFee   int64  `json:"refund_fee"`
	LeftFee     int64  `json:"left_fee"`
	WxOrderId   string `json:"wx_order_id"`
	PaidTime    int64  `json:"paid_time"`
	ProvideTime int64  `json:"provide_time"`
}

// WechatVpayQueryOrderResult mirrors the /xpay/query_order response.
type WechatVpayQueryOrderResult struct {
	ErrCode int                     `json:"errcode"`
	ErrMsg  string                  `json:"errmsg"`
	Order   *WechatVpayQueriedOrder `json:"order"`
}

// postWechatVpayApi signs (pay_sig) and POSTs an /xpay/* server API call with the
// given mini-program app's credentials. Returns the raw response body.
func postWechatVpayApi(ctx context.Context, app *model.WechatVpayApp, uri string, bodyObj any) ([]byte, error) {
	if app == nil {
		return nil, fmt.Errorf("小程序配置不存在")
	}
	appKey := app.AppKeyForEnv()
	if appKey == "" {
		return nil, fmt.Errorf("小程序 %s 未配置 AppKey", app.AppId)
	}
	accessToken, err := getWechatVpayAccessToken(ctx, app)
	if err != nil {
		return nil, err
	}
	body, err := common.Marshal(bodyObj)
	if err != nil {
		return nil, err
	}
	paySig := WechatVpayCalcPaySig(appKey, uri, string(body))
	reqUrl := fmt.Sprintf("%s%s?access_token=%s&pay_sig=%s",
		wechatVpayApiBase, uri,
		url.QueryEscape(accessToken), url.QueryEscape(paySig))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqUrl, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// QueryWechatVpayOrder calls POST /xpay/query_order (pay signature only) as the
// delivery-fallback reconciliation path, using the order's mini-program app for
// credentials. The raw response body is also returned for admin diagnostics.
func QueryWechatVpayOrder(ctx context.Context, app *model.WechatVpayApp, openid, outTradeNo string) (*WechatVpayQueryOrderResult, []byte, error) {
	if openid == "" || outTradeNo == "" {
		return nil, nil, fmt.Errorf("openid 和订单号不能为空")
	}
	if app == nil {
		return nil, nil, fmt.Errorf("订单所属小程序配置不存在")
	}
	raw, err := postWechatVpayApi(ctx, app, wechatVpayQueryOrderUri, map[string]any{
		"openid":   openid,
		"env":      app.Env,
		"order_id": outTradeNo,
	})
	if err != nil {
		return nil, raw, err
	}
	var result WechatVpayQueryOrderResult
	if err := common.Unmarshal(raw, &result); err != nil {
		return nil, raw, fmt.Errorf("解析 query_order 响应失败: %w", err)
	}
	return &result, raw, nil
}

// WechatVpayUploadItem is one item of the /xpay/start_upload_goods payload.
// Id is caller-chosen (letters/digits/_/-, <=20 chars); the same Id can be
// re-uploaded to update the dev-version item.
type WechatVpayUploadItem struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Price   int64  `json:"price"` // 单位：分
	Remark  string `json:"remark"`
	ItemUrl string `json:"item_url"` // jpg/png，微信侧会拉取校验
}

// WechatVpayUploadGoodsResult mirrors the start_upload_goods response.
type WechatVpayUploadGoodsResult struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// StartWechatVpayUploadGoods starts an item upload task (one item per call,
// uploaded to the dev version; publish is a separate API). Note: the personal
// (个人主体) doc does not list the goods APIs — whether an app may call them
// must be probed per app.
func StartWechatVpayUploadGoods(ctx context.Context, app *model.WechatVpayApp, item WechatVpayUploadItem) (*WechatVpayUploadGoodsResult, []byte, error) {
	raw, err := postWechatVpayApi(ctx, app, wechatVpayUploadGoodsUri, map[string]any{
		"upload_item": []WechatVpayUploadItem{item},
		"env":         app.Env,
	})
	if err != nil {
		return nil, raw, err
	}
	var result WechatVpayUploadGoodsResult
	if err := common.Unmarshal(raw, &result); err != nil {
		return nil, raw, fmt.Errorf("解析 start_upload_goods 响应失败: %w", err)
	}
	return &result, raw, nil
}
