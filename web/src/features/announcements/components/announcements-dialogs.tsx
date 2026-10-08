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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { handleServerError } from '@/lib/handle-server-error'

import {
  deleteAnnouncement,
  patchAnnouncementStatus,
} from '../api'
import { AnnouncementsMutateDrawer } from './announcements-mutate-drawer'
import { useAnnouncements } from './announcements-provider'

export function AnnouncementsDialogs() {
  const { open, setOpen, currentRow } = useAnnouncements()
  const isUpdate = open === 'update'

  return (
    <>
      <AnnouncementsMutateDrawer
        open={open === 'create' || isUpdate}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={isUpdate ? currentRow || undefined : undefined}
      />
      <ToggleStatusDialog />
      <DeleteAnnouncementDialog />
    </>
  )
}

export function ToggleStatusDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useAnnouncements()
  const [loading, setLoading] = useState(false)

  if (open !== 'toggle-status' || !currentRow) return null

  const isEnabled = currentRow.enabled
  const title = isEnabled ? t('Confirm disable') : t('Confirm enable')
  const description = isEnabled
    ? t(
        'After disabling, the announcement will no longer be delivered to the app. Continue?'
      )
    : t('After enabling, the announcement will be delivered to the app. Continue?')

  const handleConfirm = async () => {
    setLoading(true)
    try {
      const res = await patchAnnouncementStatus(currentRow.id, !isEnabled)
      if (res.success) {
        toast.success(
          isEnabled ? t('Has been disabled') : t('Has been enabled')
        )
        triggerRefresh()
        setOpen(null)
      } else {
        handleServerError(res)
      }
    } catch (error) {
      handleServerError(error, t('Operation failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <ConfirmDialog
      open
      onOpenChange={(v) => !v && setOpen(null)}
      title={title}
      desc={description}
      handleConfirm={handleConfirm}
      isLoading={loading}
      confirmText={isEnabled ? t('Disable') : t('Enable')}
      destructive={isEnabled}
    />
  )
}

export function DeleteAnnouncementDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useAnnouncements()
  const [loading, setLoading] = useState(false)

  if (open !== 'delete' || !currentRow) return null

  const handleConfirm = async () => {
    setLoading(true)
    try {
      const res = await deleteAnnouncement(currentRow.id)
      if (res.success) {
        toast.success(t('Deleted successfully'))
        triggerRefresh()
        setOpen(null)
      } else {
        handleServerError(res)
      }
    } catch (error) {
      handleServerError(error, t('Operation failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <ConfirmDialog
      open
      onOpenChange={(v) => !v && setOpen(null)}
      title={t('Confirm delete')}
      desc={t(
        'This announcement will be permanently deleted and will no longer be delivered to the app. Continue?'
      )}
      handleConfirm={handleConfirm}
      isLoading={loading}
      destructive
    />
  )
}
