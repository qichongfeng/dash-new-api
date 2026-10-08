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
import React, { useState } from 'react'

import useDialogState from '@/hooks/use-dialog'

import type {
  Announcement,
  AnnouncementsDialogType,
} from '../types'

type AnnouncementsContextType = {
  open: AnnouncementsDialogType | null
  setOpen: (str: AnnouncementsDialogType | null) => void
  currentRow: Announcement | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Announcement | null>>
  refreshTrigger: number
  triggerRefresh: () => void
  appFilter: string // '' = all apps
  setAppFilter: (app: string) => void
}

const AnnouncementsContext =
  React.createContext<AnnouncementsContextType | null>(null)

export function AnnouncementsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<AnnouncementsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Announcement | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const [appFilter, setAppFilter] = useState('')

  const triggerRefresh = () => setRefreshTrigger((prev) => prev + 1)

  return (
    <AnnouncementsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
        appFilter,
        setAppFilter,
      }}
    >
      {children}
    </AnnouncementsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useAnnouncements = () => {
  const ctx = React.useContext(AnnouncementsContext)
  if (!ctx) {
    throw new Error(
      'useAnnouncements has to be used within <AnnouncementsProvider>'
    )
  }
  return ctx
}
