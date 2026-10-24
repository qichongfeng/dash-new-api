import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import {
  PLAN_FORM_DEFAULTS,
  formValuesToPlanPayload,
  getPlanFormSchema,
  planToFormValues,
} from '../plan-form'
import type { SubscriptionPlan } from '../../types'

const t = ((key: string) => key) as unknown as TFunction

const basePlan: SubscriptionPlan = {
  id: 1,
  title: 'Monthly',
  price_amount: 9.9,
  duration_unit: 'month',
  duration_value: 1,
  quota_reset_period: 'never',
  enabled: true,
  sort_order: 0,
  allow_balance_pay: true,
  allow_wallet_overflow: true,
  max_purchase_per_user: 0,
  total_amount: 0,
} as unknown as SubscriptionPlan

describe('plan form currency handling', () => {
  it('defaults and schema carry no currency field so plans stay USD on the server', () => {
    expect('currency' in PLAN_FORM_DEFAULTS).toBe(false)
    expect('currency' in getPlanFormSchema(t).shape).toBe(false)
  })

  it('payload omits currency for the backend to normalize', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Monthly',
      price_amount: 9.9,
    })
    expect('currency' in payload.plan).toBe(false)
  })

  it('maps a legacy CNY plan without exposing currency in the form', () => {
    const values = planToFormValues({
      ...basePlan,
      currency: 'CNY',
    } as SubscriptionPlan)
    expect('currency' in values).toBe(false)
    expect(values.title).toBe('Monthly')
  })
})

describe('plan form wechat vpay product mapping', () => {
  it('parses the per-app product map and serializes it back to JSON', () => {
    const values = planToFormValues({
      ...basePlan,
      wechat_vpay_product_ids:
        '{"wxapp000000000001":"prod_a","wxapp000000000002":"prod_b"}',
    } as SubscriptionPlan)
    expect(values.wechat_vpay_products).toEqual({
      wxapp000000000001: 'prod_a',
      wxapp000000000002: 'prod_b',
    })

    const payload = formValuesToPlanPayload({
      ...values,
      wechat_vpay_products: { wxapp000000000001: 'prod_a' },
    })
    expect(payload.plan.wechat_vpay_product_ids).toBe(
      '{"wxapp000000000001":"prod_a"}'
    )
    expect('wechat_vpay_products' in payload.plan).toBe(false)
  })

  it('falls back to an empty map for missing or malformed JSON', () => {
    expect(planToFormValues(basePlan).wechat_vpay_products).toEqual({})
    expect(
      planToFormValues({
        ...basePlan,
        wechat_vpay_product_ids: 'not json',
      } as SubscriptionPlan).wechat_vpay_products
    ).toEqual({})
  })
})
