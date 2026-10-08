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
/** 与后端 controller/announcement_image.go 的上限保持一致 */
export const MAX_ANNOUNCEMENT_IMAGE_BYTES = 5 * 1024 * 1024

export type AnnouncementImageMediaType =
  | 'image/png'
  | 'image/jpeg'
  | 'image/gif'
  | 'image/webp'

/**
 * 按扩展名判定图片类型（浏览器对 file.type 的上报在部分平台不可靠）。
 * 非白名单扩展返回 null。
 */
export function announcementImageMediaType(
  fileName: string
): AnnouncementImageMediaType | null {
  const lower = fileName.toLowerCase()
  if (lower.endsWith('.png')) return 'image/png'
  if (lower.endsWith('.jpg') || lower.endsWith('.jpeg')) return 'image/jpeg'
  if (lower.endsWith('.gif')) return 'image/gif'
  if (lower.endsWith('.webp')) return 'image/webp'
  return null
}

export type AnnouncementImageFileFailure = 'unsupported_type' | 'too_large'

export class AnnouncementImageFileError extends Error {
  constructor(public reason: AnnouncementImageFileFailure) {
    super(reason)
  }
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  const chunk = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunk) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunk))
  }
  return btoa(binary)
}

/**
 * 编码为后端上传接口接收的 data URI；服务端会再按魔数校验并以其为准。
 */
export async function encodeAnnouncementImageFile(
  file: File
): Promise<string> {
  const mediaType = announcementImageMediaType(file.name)
  if (!mediaType) throw new AnnouncementImageFileError('unsupported_type')
  if (file.size > MAX_ANNOUNCEMENT_IMAGE_BYTES) {
    throw new AnnouncementImageFileError('too_large')
  }
  const bytes = new Uint8Array(await file.arrayBuffer())
  return `data:${mediaType};base64,${bytesToBase64(bytes)}`
}
