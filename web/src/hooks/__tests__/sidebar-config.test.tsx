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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { useSidebarConfig } from '../use-sidebar-config'
import { useSidebarData } from '../use-sidebar-data'

beforeEach(() => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
  })
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  useAuthStore.getState().auth.reset()
})

function sidebarFor(admin?: object, user?: object, canConfigure = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['status'], {
    SidebarModulesAdmin: admin ? JSON.stringify(admin) : '',
  })
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'alice',
    role: 1,
    permissions: { sidebar_settings: canConfigure },
    sidebar_modules: user ? JSON.stringify(user) : '',
  })
  function Wrapper(props: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        {props.children}
      </QueryClientProvider>
    )
  }
  const result = renderHook(
    () => useSidebarConfig(useSidebarData().navGroups),
    { wrapper: Wrapper }
  )
  return result
}

const GENERAL_TITLES = [
  'Overview',
  'Dashboard',
  'API Keys',
  'Usage Logs',
  'Audit Logs',
  'Task Logs',
]

describe('sidebar module visibility', () => {
  it('shows the console entries and the admin group by default', () => {
    const { result } = sidebarFor()
    expect(result.current.map((group) => group.id)).toEqual([
      'general',
      'admin',
    ])
    expect(
      result.current
        .find((group) => group.id === 'general')
        ?.items.map((item) => item.title)
    ).toEqual(GENERAL_TITLES)
    expect(
      result.current.find((group) => group.id === 'admin')?.items.length
    ).toBeGreaterThan(0)
  })

  it.each([
    [{ console: { enabled: false } }, undefined],
    [undefined, { console: { enabled: false } }],
  ])(
    'admin or user disablement hides every console entry (%j, %j)',
    (admin, user) => {
      const { result } = sidebarFor(admin, user)
      const titles = result.current.flatMap((group) =>
        group.items.map((item) => item.title)
      )
      for (const title of GENERAL_TITLES) {
        expect(titles).not.toContain(title)
      }
    }
  )

  it.each([
    [{ console: { enabled: true, detail: false } }, undefined],
    [undefined, { console: { enabled: true, detail: false } }],
  ])(
    'detail disablement hides only the dashboard entries (%j, %j)',
    (admin, user) => {
      const { result } = sidebarFor(admin, user)
      const titles = result.current.flatMap((group) =>
        group.items.map((item) => item.title)
      )
      expect(titles).not.toContain('Overview')
      expect(titles).not.toContain('Dashboard')
      expect(titles).toContain('API Keys')
      expect(titles).toContain('Usage Logs')
    }
  )

  it.each([
    [{ admin: { enabled: true, channel: false } }, undefined],
    [undefined, { admin: { enabled: true, setting: false } }],
  ])(
    'module-level disablement hides only that admin entry (%j, %j)',
    (admin, user) => {
      const { result } = sidebarFor(admin, user)
      const titles = result.current.flatMap((group) =>
        group.items.map((item) => item.title)
      )
      expect(titles).toEqual(expect.arrayContaining(GENERAL_TITLES))
      if (admin) {
        expect(titles).not.toContain('Channels')
        expect(titles).toContain('Users')
      } else {
        expect(titles).not.toContain('System Settings')
        expect(titles).toContain('Channels')
      }
    }
  )

  it('users without sidebar configuration permission retain the admin view', () => {
    const { result } = sidebarFor(
      undefined,
      { console: { enabled: true, detail: false } },
      false
    )
    expect(result.current.map((group) => group.id)).toEqual([
      'general',
      'admin',
    ])
  })
})
