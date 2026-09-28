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
import { describe, expect, test } from 'vitest'

import { resolveLegacyRoute } from './legacy-route'

describe('legacy frontend route migration', () => {
  test('maps former login and console routes to their current destinations', () => {
    const routes = {
      '/login': '/sign-in',
      '/forbidden': '/403',
      '/console': '/dashboard',
      '/console/channel': '/dashboard',
      '/console/removed': '/dashboard',
    }

    for (const [source, target] of Object.entries(routes)) {
      expect(resolveLegacyRoute(source)).toBe(target)
    }
  })

  test('preserves search and hash while redirecting', () => {
    expect(resolveLegacyRoute('/login?redirect=%2Fkeys#continue')).toBe(
      '/sign-in?redirect=%2Fkeys#continue'
    )
    expect(resolveLegacyRoute('/console/topup?source=email#orders')).toBe(
      '/dashboard?source=email#orders'
    )
  })

  test('does not touch current routes', () => {
    expect(resolveLegacyRoute('/dashboard')).toBe(null)
    expect(resolveLegacyRoute('/api/status')).toBe(null)
  })
})
