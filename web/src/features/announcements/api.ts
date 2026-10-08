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
import { api } from '@/lib/api'

import type {
  Announcement,
  AnnouncementImageUploadResult,
  AnnouncementPageData,
  AnnouncementPayload,
  ApiResponse,
} from './types'

export async function getAnnouncements(params?: {
  app?: string
  p?: number
  page_size?: number
}): Promise<ApiResponse<AnnouncementPageData>> {
  const res = await api.get('/api/announcement/admin/list', { params })
  return res.data
}

export async function createAnnouncement(
  data: AnnouncementPayload
): Promise<ApiResponse<Announcement>> {
  const res = await api.post('/api/announcement/admin/', data)
  return res.data
}

export async function updateAnnouncement(
  id: number,
  data: AnnouncementPayload
): Promise<ApiResponse<Announcement>> {
  const res = await api.put(`/api/announcement/admin/${id}`, data)
  return res.data
}

export async function patchAnnouncementStatus(
  id: number,
  enabled: boolean
): Promise<ApiResponse> {
  const res = await api.patch(`/api/announcement/admin/${id}/status`, {
    enabled,
  })
  return res.data
}

export async function deleteAnnouncement(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/announcement/admin/${id}`)
  return res.data
}

export async function uploadAnnouncementImage(
  dataUri: string
): Promise<ApiResponse<AnnouncementImageUploadResult>> {
  const res = await api.post('/api/announcement/admin/image', {
    image: dataUri,
  })
  return res.data
}
