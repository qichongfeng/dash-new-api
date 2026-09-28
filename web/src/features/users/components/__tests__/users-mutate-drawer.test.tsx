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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { UsersMutateDrawer } from '../users-mutate-drawer'
import { UsersProvider } from '../users-provider'

function renderCreateDrawer() {
  vi.spyOn(api, 'get').mockImplementation(async () => ({
    data: { success: true, data: ['default'] },
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <UsersProvider>
        <UsersMutateDrawer open onOpenChange={() => undefined} />
      </UsersProvider>
    </QueryClientProvider>
  )
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

it('create drawer renders no role field when only common users can be created', async () => {
  renderCreateDrawer()
  await screen.findByText('Basic Information')
  expect(screen.queryByText('Role')).not.toBeInTheDocument()
  expect(screen.queryByText('Common User')).not.toBeInTheDocument()
})

it('create payload omits role and admin_permissions when submitting a new user', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  renderCreateDrawer()
  await userEvent.type(await screen.findByLabelText(/Username/i), 'new-user')
  await userEvent.type(screen.getByLabelText(/Password/i), 'password-123')
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith('/api/user/', {
      username: 'new-user',
      display_name: 'new-user',
      password: 'password-123',
    })
  )
  expect(post.mock.calls[0][1]).not.toHaveProperty('role')
  expect(post.mock.calls[0][1]).not.toHaveProperty('admin_permissions')
})
