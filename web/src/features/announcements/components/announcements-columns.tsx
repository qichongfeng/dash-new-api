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
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { formatTimestamp } from '@/lib/format'

import type { AnnouncementListItem } from '../types'
import { DataTableRowActions } from './data-table-row-actions'

const TYPE_LABEL_KEYS: Record<AnnouncementListItem['type'], string> = {
  default: 'Default',
  ongoing: 'Ongoing',
  success: 'Success',
  warning: 'Warning',
  error: 'Error',
}

const TYPE_BADGE_VARIANTS: Record<AnnouncementListItem['type'], string> = {
  default: 'neutral',
  ongoing: 'info',
  success: 'success',
  warning: 'warning',
  error: 'danger',
}

export function useAnnouncementsColumns(): ColumnDef<AnnouncementListItem>[] {
  const { t } = useTranslation()

  return useMemo(
    (): ColumnDef<AnnouncementListItem>[] => [
      {
        accessorKey: 'id',
        header: t('ID'),
        meta: { mobileHidden: true },
        cell: ({ row }) => <TableId value={row.original.id} />,
        size: 60,
      },
      {
        accessorKey: 'app',
        header: t('App'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <StatusBadge
            label={row.original.app}
            variant='neutral'
            copyable={false}
            className='-ml-1.5'
          />
        ),
        size: 110,
      },
      {
        accessorKey: 'title',
        header: t('Title'),
        meta: { mobileTitle: true },
        cell: ({ row }) => (
          <div className='max-w-full min-w-0 truncate font-medium'>
            {row.original.title}
          </div>
        ),
        size: 260,
      },
      {
        accessorKey: 'type',
        header: t('Type'),
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const typ = row.original.type
          return (
            <StatusBadge
              label={t(TYPE_LABEL_KEYS[typ])}
              variant={
                TYPE_BADGE_VARIANTS[typ] as
                  | 'neutral'
                  | 'info'
                  | 'success'
                  | 'warning'
                  | 'danger'
              }
              copyable={false}
              className='-ml-1.5'
            />
          )
        },
        size: 90,
      },
      {
        accessorKey: 'publish_time',
        header: t('Publish Date'),
        cell: ({ row }) => {
          const a = row.original
          const scheduled = a.publish_time * 1000 > Date.now()
          return (
            <div className='flex flex-col gap-1'>
              <span className='text-muted-foreground'>
                {formatTimestamp(a.publish_time)}
              </span>
              {scheduled ? (
                <StatusBadge
                  label={t('Scheduled')}
                  variant='warning'
                  copyable={false}
                  className='-ml-1.5 w-fit'
                />
              ) : null}
            </div>
          )
        },
        size: 150,
      },
      {
        accessorKey: 'enabled',
        header: t('Status'),
        meta: { mobileBadge: true },
        cell: ({ row }) =>
          row.original.enabled ? (
            <StatusBadge
              label={t('Enable')}
              variant='success'
              copyable={false}
              className='-ml-1.5'
            />
          ) : (
            <StatusBadge
              label={t('Disable')}
              variant='neutral'
              copyable={false}
              className='-ml-1.5'
            />
          ),
        size: 80,
      },
      {
        accessorKey: 'created_at',
        header: t('Created At'),
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <span className='text-muted-foreground'>
            {formatTimestamp(row.original.created_at)}
          </span>
        ),
        size: 150,
      },
      {
        id: 'actions',
        header: () => t('Actions'),
        cell: ({ row }) => <DataTableRowActions row={row} />,
        meta: { pinned: 'right' as const },
      },
    ],
    [t]
  )
}
