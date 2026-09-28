package setting

// WeChat mini-program virtual payment (个人主体虚拟支付, 道具直购) configuration.
// The provider is enabled once AppID + OfferID + AppKey are populated (no separate
// Enabled flag, matching the other gateways). PushToken is the Token configured in
// the MP console under 开发管理-消息推送, used to verify delivery/refund push
// signatures. Env selects 现网(0)/沙箱(1); personal-subject mini programs use 0 and
// the 现网 AppKey, so SandboxAppKey is only read when Env = 1.
var (
	WechatVpayAppId         string
	WechatVpayAppSecret     string
	WechatVpayOfferId       string
	WechatVpayAppKey        string
	WechatVpaySandboxAppKey string
	WechatVpayPushToken     string
	WechatVpayEnv           int = 0
)
