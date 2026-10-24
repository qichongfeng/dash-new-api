package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWechatVpayMigrationTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Option{}, &WechatVpayApp{}))
	// 历史库才有旧单值列；新库不创建，这里手工补出历史形态
	if !DB.Migrator().HasColumn("subscription_plans", "wechat_vpay_product_id") {
		require.NoError(t, DB.Exec("ALTER TABLE subscription_plans ADD COLUMN `wechat_vpay_product_id` varchar(128) DEFAULT ''").Error)
	}
	t.Cleanup(func() {
		DB.Exec("DELETE FROM options")
		DB.Exec("DELETE FROM wechat_vpay_apps")
		DB.Exec("DELETE FROM subscription_orders")
		DB.Exec("UPDATE subscription_plans SET wechat_vpay_product_ids = '', wechat_vpay_product_id = ''")
	})
}

func insertWechatVpayLegacyOptions(t *testing.T) {
	t.Helper()
	legacy := map[string]string{
		"WechatVpayAppId":         "wxlegacy000000001",
		"WechatVpayAppSecret":     "legacy-secret",
		"WechatVpayOfferId":       "legacy-offer",
		"WechatVpayAppKey":        "legacy-appkey",
		"WechatVpaySandboxAppKey": "legacy-sandbox-key",
		"WechatVpayPushToken":     "legacy-push-token",
		"WechatVpayEnv":           "0",
	}
	for key, value := range legacy {
		require.NoError(t, DB.Create(&Option{Key: key, Value: value}).Error)
	}
}

func TestMigrateWechatVpayLegacyOptions(t *testing.T) {
	setupWechatVpayMigrationTest(t)
	insertWechatVpayLegacyOptions(t)

	// 历史订单（无归属小程序）与无关订单各一
	vpayOrder := &SubscriptionOrder{
		UserId: 1, PlanId: 1, TradeNo: "WVP-legacy-1",
		PaymentProvider: PaymentProviderWechatVpay, PaymentMethod: PaymentMethodWechatVpay,
		Status: common.TopUpStatusPending,
	}
	require.NoError(t, vpayOrder.Insert())
	otherOrder := &SubscriptionOrder{
		UserId: 1, PlanId: 1, TradeNo: "SUB-other-1",
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, otherOrder.Insert())

	// 历史套餐：旧单值道具 ID
	plan := &SubscriptionPlan{Title: "legacy plan", PriceAmount: 1, Currency: "USD", Enabled: true}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, DB.Exec("UPDATE subscription_plans SET wechat_vpay_product_id = 'prod_legacy' WHERE id = ?", plan.Id).Error)

	require.NoError(t, migrateWechatVpayLegacyOptions())

	// 配置迁移为小程序一行，凭证原样保留
	app, err := GetWechatVpayAppByAppId("wxlegacy000000001")
	require.NoError(t, err)
	require.NotNil(t, app)
	assert.Equal(t, "legacy-offer", app.OfferId)
	assert.Equal(t, "legacy-appkey", app.AppKey)
	assert.Equal(t, "legacy-sandbox-key", app.SandboxAppKey)
	assert.Equal(t, "legacy-push-token", app.PushToken)
	assert.Equal(t, "legacy-secret", app.AppSecret)
	assert.True(t, app.Enabled)
	assert.True(t, app.SigningConfigured())
	assert.True(t, app.PushConfigured())

	// 订单归属回填；无关订单不动
	got := GetSubscriptionOrderByTradeNo("WVP-legacy-1")
	require.NotNil(t, got)
	assert.Equal(t, "wxlegacy000000001", got.ProviderAppId)
	gotOther := GetSubscriptionOrderByTradeNo("SUB-other-1")
	require.NotNil(t, gotOther)
	assert.Equal(t, "", gotOther.ProviderAppId)

	// 套餐旧单值迁为映射 JSON，可按小程序解析
	var gotPlan SubscriptionPlan
	require.NoError(t, DB.First(&gotPlan, "id = ?", plan.Id).Error)
	assert.Equal(t, "prod_legacy", gotPlan.WechatVpayProductIdForApp("wxlegacy000000001"))
	assert.Equal(t, "", gotPlan.WechatVpayProductIdForApp("wxother0000000002"))

	// 旧 option 行已删除
	var remaining int64
	require.NoError(t, DB.Model(&Option{}).Where(commonKeyCol+" IN ?", wechatVpayLegacyOptionKeys).Count(&remaining).Error)
	assert.Zero(t, remaining)

	// 幂等：重复执行不报错、不重复建行
	require.NoError(t, migrateWechatVpayLegacyOptions())
	apps, err := GetWechatVpayApps()
	require.NoError(t, err)
	assert.Len(t, apps, 1)
}

func TestMigrateWechatVpayLegacyOptionsSkipsWhenNothingConfigured(t *testing.T) {
	setupWechatVpayMigrationTest(t)

	require.NoError(t, migrateWechatVpayLegacyOptions())

	apps, err := GetWechatVpayApps()
	require.NoError(t, err)
	assert.Empty(t, apps)
}

func TestDeleteWechatVpayAppBlockedByPendingOrders(t *testing.T) {
	setupWechatVpayMigrationTest(t)
	app := &WechatVpayApp{AppId: "wxdel000000000001", Name: "del", OfferId: "o", AppKey: "k", Enabled: true}
	require.NoError(t, CreateWechatVpayApp(app))

	err := DeleteWechatVpayAppById(app.Id)
	require.NoError(t, err)

	app2 := &WechatVpayApp{AppId: "wxdel000000000002", Name: "del2", OfferId: "o", AppKey: "k", Enabled: true}
	require.NoError(t, CreateWechatVpayApp(app2))
	pending := &SubscriptionOrder{
		UserId: 1, PlanId: 1, TradeNo: "WVP-del-2",
		PaymentProvider: PaymentProviderWechatVpay, PaymentMethod: PaymentMethodWechatVpay,
		ProviderAppId: app2.AppId, Status: common.TopUpStatusPending,
	}
	require.NoError(t, pending.Insert())

	err = DeleteWechatVpayAppById(app2.Id)
	require.Error(t, err)
	stillThere, err := GetWechatVpayAppById(app2.Id)
	require.NoError(t, err)
	require.NotNil(t, stillThere)

	// 订单终结后可删除
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Where("trade_no = ?", "WVP-del-2").
		Update("status", common.TopUpStatusSuccess).Error)
	require.NoError(t, DeleteWechatVpayAppById(app2.Id))
}
