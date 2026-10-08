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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { AnnouncementsProvider } from './components/announcements-provider'
import { AnnouncementsPrimaryButtons } from './components/announcements-primary-buttons'
import { AnnouncementsTable } from './components/announcements-table'
import { AnnouncementsDialogs } from './components/announcements-dialogs'

function AnnouncementsContent() {
  const { t } = useTranslation()
  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Announcement Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <AnnouncementsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            <div className='min-h-0 flex-1'>
              <AnnouncementsTable />
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <AnnouncementsDialogs />
    </>
  )
}

export function Announcements() {
  return (
    <AnnouncementsProvider>
      <AnnouncementsContent />
    </AnnouncementsProvider>
  )
}
