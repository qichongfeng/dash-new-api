package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Renewal anchoring: a new grant of the same plan starts at the latest
// unexpired active row's EndTime, so repeat purchases stack serially.

func TestCreateUserSubscriptionRenewsAfterActiveSamePlanRow(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9401,
		Title:            "Renew Plan",
		PriceAmount:      6,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      5000,
		QuotaResetPeriod: SubscriptionResetMonthly,
	}
	firstEnd := now + 10*24*3600
	seedSubscriptionResetSub(t, &UserSubscription{
		Id: 9411, UserId: 401, PlanId: plan.Id,
		AmountTotal: 5000, StartTime: now - 20*24*3600, EndTime: firstEnd,
		Status: "active", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(),
	})

	// An active row of a different plan must not anchor.
	seedSubscriptionResetSub(t, &UserSubscription{
		Id: 9412, UserId: 401, PlanId: 9999,
		AmountTotal: 100, StartTime: now - 24*3600, EndTime: now + 90*24*3600,
		Status: "active", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(),
	})

	sub, err := CreateUserSubscriptionFromPlanTx(DB, 401, plan, "order", "RENEW1")
	require.NoError(t, err)
	wantEnd, err := calcPlanEndTime(time.Unix(firstEnd, 0), plan)
	require.NoError(t, err)
	assert.Equal(t, firstEnd, sub.StartTime, "renewal row must start at the same-plan row's end")
	assert.Equal(t, wantEnd, sub.EndTime)
	assert.Equal(t, firstEnd, sub.LastResetTime, "reset cycle is anchored to the effective start")
	assert.Equal(t, calcNextResetTime(time.Unix(firstEnd, 0), plan, sub.EndTime), sub.NextResetTime)

	// Chain: a third purchase anchors to the renewal row's end, not the first row's.
	third, err := CreateUserSubscriptionFromPlanTx(DB, 401, plan, "order", "RENEW2")
	require.NoError(t, err)
	assert.Equal(t, sub.EndTime, third.StartTime)
}

func TestCreateUserSubscriptionWithoutAnchorStartsNow(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id: 9402, Title: "Fresh Plan", DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		TotalAmount: 100, QuotaResetPeriod: SubscriptionResetNever,
	}
	// Expired-by-time and cancelled rows never anchor, even with a future end_time.
	seedSubscriptionResetSub(t, &UserSubscription{
		Id: 9421, UserId: 402, PlanId: plan.Id, StartTime: now - 2*24*3600, EndTime: now + 5*24*3600,
		Status: "cancelled", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(),
	})
	seedSubscriptionResetSub(t, &UserSubscription{
		Id: 9422, UserId: 402, PlanId: plan.Id, StartTime: now - 2*24*3600, EndTime: now - 1,
		Status: "active", CreatedAt: common.GetTimestamp(), UpdatedAt: common.GetTimestamp(),
	})

	before := GetDBTimestamp()
	sub, err := CreateUserSubscriptionFromPlanTx(DB, 402, plan, "order", "FRESH1")
	after := GetDBTimestamp()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, sub.StartTime, before)
	assert.LessOrEqual(t, sub.StartTime, after)
	assert.Zero(t, sub.NextResetTime)
}

func TestCompleteSubscriptionOrderRenewalRefundRollsBackOnlyLatestRow(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 431, 0)
	plan := &SubscriptionPlan{
		Id: 9403, Title: "Guard Renew Plan", PriceAmount: 9.99,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 1000,
	}
	require.NoError(t, DB.Create(plan).Error)
	insertSubscriptionOrderForPaymentGuardTest(t, "RENEWORDER1", 431, plan.Id, PaymentProviderWechatVpay)
	insertSubscriptionOrderForPaymentGuardTest(t, "RENEWORDER2", 431, plan.Id, PaymentProviderWechatVpay)

	require.NoError(t, CompleteSubscriptionOrder("RENEWORDER1", "<xml></xml>", PaymentProviderWechatVpay, PaymentMethodWechatVpay))
	first := findSubscriptionBySourceTradeNoForGuardTest(t, "RENEWORDER1")
	firstEnd := first.EndTime

	require.NoError(t, CompleteSubscriptionOrder("RENEWORDER2", "<xml></xml>", PaymentProviderWechatVpay, PaymentMethodWechatVpay))
	second := findSubscriptionBySourceTradeNoForGuardTest(t, "RENEWORDER2")
	require.Equal(t, firstEnd, second.StartTime, "second fulfillment must stack after the first row")

	cancelledId, err := CancelUserSubscriptionByTradeNo("RENEWORDER2")
	require.NoError(t, err)
	assert.Equal(t, second.Id, cancelledId)

	second = findSubscriptionBySourceTradeNoForGuardTest(t, "RENEWORDER2")
	assert.Equal(t, "cancelled", second.Status)
	assert.LessOrEqual(t, second.EndTime, time.Now().Unix())

	first = findSubscriptionBySourceTradeNoForGuardTest(t, "RENEWORDER1")
	assert.Equal(t, "active", first.Status)
	assert.Equal(t, firstEnd, first.EndTime)
}

func TestCreateUserSubscriptionRenewalStillCountsPurchaseLimit(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id: 9404, Title: "Limited Plan", DurationUnit: SubscriptionDurationDay, DurationValue: 7,
		TotalAmount: 100, MaxPurchasePerUser: 2,
	}
	for range 2 {
		_, err := CreateUserSubscriptionFromPlanTx(DB, 433, plan, "order", "")
		require.NoError(t, err)
	}
	_, err := CreateUserSubscriptionFromPlanTx(DB, 433, plan, "order", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已达到该套餐购买上限")
}
