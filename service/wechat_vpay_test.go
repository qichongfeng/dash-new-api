package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Official signature vectors from the WeChat virtual-payment docs (签名详解 /
// 参考的 Python 脚本). They pin the exact concatenation and HMAC construction.
func TestWechatVpayCalcPaySig_OfficialVector(t *testing.T) {
	signData := `{"openid": "xxx", "user_ip": "127.0.0.1", "env": 0}`
	got := WechatVpayCalcPaySig("12345", "/xpay/query_user_balance", signData)
	assert.Equal(t, "c37809f27c6d7fd1837ad2500a04512b66b34fd793a39a385fade56dca89a4b5", got)
}

func TestWechatVpayCalcUserSig_OfficialVector(t *testing.T) {
	signData := `{"openid": "xxx", "user_ip": "127.0.0.1", "env": 0}`
	got := WechatVpayCalcUserSig("9hAb/NEYUlkaMBEsmFgzig==", signData)
	assert.Equal(t, "089d9e8dc5d308977360c4b79ec600a93d736802802a807d634192328032f6c7", got)
}

func TestBuildWechatVpayPayData(t *testing.T) {
	appKey := "app-key"
	sessionKey := "session-key"
	sign := &WechatVpaySignData{
		OfferId:      "offer-1",
		BuyQuantity:  1,
		Env:          0,
		CurrencyType: "CNY",
		ProductId:    "prod-1",
		GoodsPrice:   100,
		OutTradeNo:   "WVP1700000000000abcdefghi",
		Attach:       "WVP1700000000000abcdefghi",
	}
	payData, err := BuildWechatVpayPayData(appKey, sign, sessionKey)
	require.NoError(t, err)

	assert.Equal(t, WechatVpayPayMode, payData.Mode)
	// The signed payload must be exactly the string returned to the mini-program,
	// and both signatures must be recomputable from it with the documented uris.
	assert.Equal(t, WechatVpayCalcPaySig(appKey, "requestVirtualPayment", payData.SignData), payData.PaySig)
	assert.Equal(t, WechatVpayCalcUserSig(sessionKey, payData.SignData), payData.Signature)
	assert.Contains(t, payData.SignData, `"offerId":"offer-1"`)
	assert.Contains(t, payData.SignData, `"goodsPrice":100`)
	assert.Contains(t, payData.SignData, `"outTradeNo":"WVP1700000000000abcdefghi"`)
}

const wechatVpayDeliverPushXML = `<xml>
	<ToUserName><![CDATA[gh_raw_id]]></ToUserName>
	<FromUserName><![CDATA[wx_official_openid]]></FromUserName>
	<CreateTime>1759000000</CreateTime>
	<MsgType>event</MsgType>
	<Event>xpay_goods_deliver_notify</Event>
	<OpenId><![CDATA[user_openid]]></OpenId>
	<OutTradeNo><![CDATA[WVP1700000000000abcdefghi]]></OutTradeNo>
	<Env>0</Env>
	<WeChatPayInfo>
		<MchOrderNo><![CDATA[wx_order_id_1]]></MchOrderNo>
		<TransactionId><![CDATA[wxpay_txn_1]]></TransactionId>
		<PaidTime>1759000100</PaidTime>
	</WeChatPayInfo>
	<GoodsInfo>
		<ProductId><![CDATA[prod-1]]></ProductId>
		<Quantity>1</Quantity>
		<OrigPrice>100</OrigPrice>
		<ActualPrice>100</ActualPrice>
		<Attach><![CDATA[WVP1700000000000abcdefghi]]></Attach>
	</GoodsInfo>
</xml>`

const wechatVpayRefundPushXML = `<xml>
	<ToUserName><![CDATA[gh_raw_id]]></ToUserName>
	<FromUserName><![CDATA[wx_official_openid]]></FromUserName>
	<CreateTime>1759000200</CreateTime>
	<MsgType>event</MsgType>
	<Event>xpay_refund_notify</Event>
	<OpenId><![CDATA[user_openid]]></OpenId>
	<WxRefundId><![CDATA[wx_refund_1]]></WxRefundId>
	<MchRefundId><![CDATA[mch_refund_1]]></MchRefundId>
	<WxOrderId><![CDATA[wx_order_id_1]]></WxOrderId>
	<MchOrderId><![CDATA[WVP1700000000000abcdefghi]]></MchOrderId>
	<RefundFee>100</RefundFee>
	<RetCode>0</RetCode>
	<RetMsg><![CDATA[ok]]></RetMsg>
	<RefundSuccTimestamp>1759000200</RefundSuccTimestamp>
</xml>`

func TestParseWechatVpayPush(t *testing.T) {
	t.Run("deliver push", func(t *testing.T) {
		msg, err := ParseWechatVpayPush([]byte(wechatVpayDeliverPushXML))
		require.NoError(t, err)
		assert.Equal(t, WechatVpayEventGoodsDeliver, msg.Event)
		assert.Equal(t, "user_openid", msg.OpenId)
		assert.Equal(t, "WVP1700000000000abcdefghi", msg.OutTradeNo)
		assert.Equal(t, 0, msg.Env)
		require.NotNil(t, msg.WeChatPayInfo)
		assert.Equal(t, "wx_order_id_1", msg.WeChatPayInfo.MchOrderNo)
		assert.Equal(t, int64(1759000100), msg.WeChatPayInfo.PaidTime)
		require.NotNil(t, msg.GoodsInfo)
		assert.Equal(t, "prod-1", msg.GoodsInfo.ProductId)
		assert.Equal(t, 1, msg.GoodsInfo.Quantity)
		assert.Equal(t, int64(100), msg.GoodsInfo.ActualPrice)
	})
	t.Run("refund push", func(t *testing.T) {
		msg, err := ParseWechatVpayPush([]byte(wechatVpayRefundPushXML))
		require.NoError(t, err)
		assert.Equal(t, WechatVpayEventRefund, msg.Event)
		assert.Equal(t, "WVP1700000000000abcdefghi", msg.MchOrderId)
		assert.Equal(t, int64(100), msg.RefundFee)
		assert.Equal(t, 0, msg.RetCode)
	})
	t.Run("other event passes through", func(t *testing.T) {
		msg, err := ParseWechatVpayPush([]byte(`<xml><Event>xpay_complaint_notify</Event></xml>`))
		require.NoError(t, err)
		assert.Equal(t, "xpay_complaint_notify", msg.Event)
	})
	t.Run("invalid xml", func(t *testing.T) {
		_, err := ParseWechatVpayPush([]byte(`not xml`))
		require.Error(t, err)
	})
}

func TestVerifyWechatVpayPushSignature(t *testing.T) {
	// sha1(sorted("testtoken","1234567890","nonce","")) = 160a850a...
	assert.True(t, VerifyWechatVpayPushSignature("testtoken", "1234567890", "nonce", "", "160a850aa1c947c71fafb8c3a4efba532a0bcafb"))
	// URL verification variant: the echostr participates in the sort.
	// sha1(sorted("testtoken","1234567890","nonce","echostr123")) = c705df3d...
	assert.True(t, VerifyWechatVpayPushSignature("testtoken", "1234567890", "nonce", "echostr123", "c705df3d20326b784340fdc96785abf6f5ac860e"))
	assert.False(t, VerifyWechatVpayPushSignature("testtoken", "1234567890", "nonce", "", "deadbeef"))
	assert.False(t, VerifyWechatVpayPushSignature("", "1234567890", "nonce", "", "160a850aa1c947c71fafb8c3a4efba532a0bcafb"))
	assert.False(t, VerifyWechatVpayPushSignature("testtoken", "1234567890", "nonce", "", ""))
}

func TestWechatVpayOrderStatusIsPaid(t *testing.T) {
	assert.True(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusPaid))
	assert.True(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusDelivering))
	assert.True(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusDelivered))
	assert.False(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusCreated))
	assert.False(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusRefunded))
	assert.False(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusUserRefunded))
	assert.False(t, WechatVpayOrderStatusIsPaid(WechatVpayOrderStatusClosed))
}

func TestWechatVpayGoodsPriceFen(t *testing.T) {
	testCases := []struct {
		name         string
		priceAmount  float64
		exchangeRate float64
		expectedFen  int64
		expectErr    bool
	}{
		{name: "whole conversion", priceAmount: 10, exchangeRate: 7.3, expectedFen: 7300},
		{name: "exact fen conversion", priceAmount: 9.9, exchangeRate: 7, expectedFen: 6930},
		{name: "float representation stays within tolerance", priceAmount: 9.9, exchangeRate: 7.3, expectedFen: 7227},
		{name: "conversion between fen is rejected", priceAmount: 1.01, exchangeRate: 7.3, expectErr: true},
		{name: "non-zero fen fraction is rejected", priceAmount: 0.99, exchangeRate: 7.3, expectErr: true},
		{name: "zero price is rejected", priceAmount: 0, exchangeRate: 7.3, expectErr: true},
		{name: "negative price is rejected", priceAmount: -1, exchangeRate: 7.3, expectErr: true},
		{name: "zero rate is rejected", priceAmount: 10, exchangeRate: 0, expectErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fen, err := WechatVpayGoodsPriceFen(tc.priceAmount, tc.exchangeRate)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedFen, fen)
		})
	}
}
