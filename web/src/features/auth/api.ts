/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import axios from 'axios'

import { api, refreshAuthentication, type RefreshOutcome } from '@/lib/api'
import { AuthOperationError } from '@/lib/secure-verification'
import { getServerErrorMessageKey } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import {
  clearPasswordEncryptionCache,
  encryptPassword,
} from './lib/password-encryption'
import { getAffiliateCode } from './lib/storage'
import type { VerificationOperation } from './secure-verification/types'
import type {
  LoginPayload,
  LoginResponse,
  Login2FAResponse,
  TwoFAPayload,
  ApiResponse,
} from './types'

// ============================================================================
// Authentication APIs
// ============================================================================

// ----------------------------------------------------------------------------
// Login & Logout
// ----------------------------------------------------------------------------

// User login with username and password
export async function login(payload: LoginPayload): Promise<LoginResponse> {
  const turnstile = payload.turnstile ?? ''
  try {
    let passwordFields:
      | { password: string }
      | { password_encrypted: string; encryption_key_id: string }
    if (payload.passwordEncryptionEnabled) {
      const encryptedPassword = await encryptPassword(payload.password)
      passwordFields = {
        password_encrypted: encryptedPassword.password_encrypted,
        encryption_key_id: encryptedPassword.encryption_key_id,
      }
    } else {
      passwordFields = { password: payload.password }
    }
    const res = await api.post<LoginResponse>(
      `/api/user/login?turnstile=${turnstile}`,
      {
        username: payload.username,
        ...passwordFields,
      },
      { skipAuthRefresh: true }
    )
    if (payload.passwordEncryptionEnabled && !res.data?.success) {
      clearPasswordEncryptionCache()
    }
    return res.data
  } catch (error: unknown) {
    if (payload.passwordEncryptionEnabled) {
      clearPasswordEncryptionCache()
    }
    throw error
  }
}

// Two-factor authentication login
export async function login2fa(payload: TwoFAPayload) {
  const res = await api.post<Login2FAResponse>('/api/user/login/2fa', payload, {
    skipAuthRefresh: true,
  })
  return res.data
}

interface LogoutRuntime {
  getExpectedSID: () => string | undefined
  request: (expectedSID?: string) => Promise<ApiResponse>
  refresh: () => Promise<RefreshOutcome>
}

export async function executeLogout(
  runtime: LogoutRuntime,
  allowMismatchRecovery = true
): Promise<ApiResponse> {
  try {
    return await runtime.request(runtime.getExpectedSID())
  } catch (error: unknown) {
    const code = axios.isAxiosError(error)
      ? error.response?.data?.code
      : undefined
    if (
      allowMismatchRecovery &&
      axios.isAxiosError(error) &&
      error.response?.status === 409 &&
      code === 'AUTH_SESSION_MISMATCH'
    ) {
      const outcome = await runtime.refresh()
      if (outcome.kind === 'authenticated') {
        return executeLogout(runtime, false)
      }
      if (outcome.kind === 'anonymous') {
        return { success: true, message: '' }
      }
    }
    throw error
  }
}

// User logout
export async function logout(): Promise<ApiResponse> {
  return executeLogout({
    getExpectedSID: () => useAuthStore.getState().auth.session?.sid,
    request: async (sid) => {
      const res = await api.post('/api/user/auth/logout', undefined, {
        headers: sid ? { 'X-Auth-Session': sid } : undefined,
        skipAuthRefresh: true,
        skipErrorHandler: true,
      })
      return res.data
    },
    refresh: refreshAuthentication,
  })
}

// ----------------------------------------------------------------------------
// OAuth
// ----------------------------------------------------------------------------

// Get OAuth state for CSRF protection
export async function createOAuthAuthorization(
  provider: string,
  intent: 'login' | 'bind' | 'verify',
  operation?: VerificationOperation,
  signal?: AbortSignal,
  proofToken?: string
): Promise<{ state: string; authorizationUrl?: string }> {
  const aff = intent === 'login' ? getAffiliateCode() : ''
  const res = await api.post(
    '/api/oauth/state',
    {
      provider,
      intent,
      aff: aff || undefined,
      scope: operation?.scope,
      ...(operation?.context ? { context: operation.context } : {}),
    },
    {
      skipAuthRefresh: intent === 'login',
      ...(proofToken ? { headers: { 'X-Security-Proof': proofToken } } : {}),
      singleUseAuthorization: intent === 'bind',
      signal,
      skipBusinessError: true,
      skipErrorHandler: true,
    }
  )
  if (res.data?.success) {
    if (typeof res.data.data === 'string') return { state: res.data.data }
    if (typeof res.data.data?.flow_token === 'string') {
      return {
        state: res.data.data.flow_token,
        authorizationUrl: res.data.data.authorization_url,
      }
    }
  }
  throw new AuthOperationError(
    getServerErrorMessageKey(res.data) ||
      res.data?.message ||
      'Failed to initialize OAuth',
    res.data?.code
  )
}

export async function createOAuthFlow(
  provider: string,
  intent: 'login' | 'bind' | 'verify',
  operation?: VerificationOperation,
  signal?: AbortSignal
): Promise<string> {
  return (await createOAuthAuthorization(provider, intent, operation, signal))
    .state
}

// Confirm an authenticated, server-owned email binding flow.
export async function bindEmail(
  flowToken: string,
  newCode: string,
  oldCode = '',
  signal?: AbortSignal
): Promise<ApiResponse> {
  const res = await api.post(
    '/api/oauth/email/bind',
    {
      flow_token: flowToken,
      new_code: newCode,
      old_code: oldCode,
    },
    { singleUseAuthorization: true, signal }
  )
  return res.data
}
