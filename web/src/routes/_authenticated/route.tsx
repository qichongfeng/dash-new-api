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
import { createFileRoute, redirect } from '@tanstack/react-router'
import { t } from 'i18next'
import { toast } from 'sonner'

import { AuthenticatedLayout } from '@/components/layout'
import {
  clearAuthentication,
  resolveAuthentication,
} from '@/lib/auth-session'
import { hasAdminRole } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location }) => {
    // The root guard may have skipped its refresh because no session hint was
    // present. That skip is an optimization for public pages and must not
    // decide a protected route, so resolve against the server before
    // redirecting. An in-memory session returns without a request.
    await resolveAuthentication()

    const { auth } = useAuthStore.getState()

    // The dashboard is admin-only. Clear the client session so the sign-in
    // page does not bounce an authenticated non-admin back into this guard.
    if (!auth.user || !auth.accessToken || !hasAdminRole(auth.user.role)) {
      if (auth.user) {
        clearAuthentication(false)
        toast.error(t('Sign-in is restricted to administrators'))
        throw redirect({ to: '/sign-in' })
      }
      throw redirect({
        to: '/sign-in',
        search: { redirect: location.href },
      })
    }
  },
  component: AuthenticatedLayout,
})
