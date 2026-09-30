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
