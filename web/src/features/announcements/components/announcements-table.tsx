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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getAnnouncements } from '../api'
import { useAnnouncementsColumns } from './announcements-columns'
import { useAnnouncements } from './announcements-provider'

export function AnnouncementsTable() {
  const { t } = useTranslation()
  const columns = useAnnouncementsColumns()
  const { refreshTrigger, appFilter } = useAnnouncements()

  const { data, isLoading } = useQuery({
    queryKey: ['admin-announcements', appFilter, refreshTrigger],
    queryFn: async () => {
      // 公告量小：一页拉满（服务端上限 100），客户端直接展示
      const result = requireServerSuccess(
        await getAnnouncements({ app: appFilter || undefined, page_size: 100 })
      )
      return result.data?.items || []
    },
    placeholderData: (prev) => prev,
  })

  const announcements = useMemo(() => data || [], [data])

  const { table } = useDataTable({
    data: announcements,
    columns,
    withFilteredRowModel: false,
    withFacetedRowModel: false,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      emptyTitle={t('No announcements yet')}
      emptyDescription={t(
        'Click "Create Announcement" to publish your first announcement'
      )}
      skeletonKeyPrefix='announcements-skeleton'
      applyHeaderSize
    />
  )
}
