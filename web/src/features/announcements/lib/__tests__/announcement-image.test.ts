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

import {
  announcementImageMediaType,
  encodeAnnouncementImageFile,
  MAX_ANNOUNCEMENT_IMAGE_BYTES,
} from '../announcement-image'

function imageFile(name: string, size = 16): File {
  return new File([new Uint8Array(size)], name)
}

describe('announcementImageMediaType', () => {
  test.each([
    ['a.PNG', 'image/png'],
    ['photo.jpg', 'image/jpeg'],
    ['photo.JPEG', 'image/jpeg'],
    ['anim.gif', 'image/gif'],
    ['pic.webp', 'image/webp'],
  ])('maps %s by extension case-insensitively', (name, expected) => {
    expect(announcementImageMediaType(name)).toBe(expected)
  })

  test.each(['icon.svg', 'movie.mp4', 'file', 'doc.pdf.txt'])(
    'rejects %s outside the whitelist',
    (name) => {
      expect(announcementImageMediaType(name)).toBeNull()
    }
  )
})

describe('encodeAnnouncementImageFile', () => {
  test('encodes a whitelisted file as a base64 data URI', async () => {
    const uri = await encodeAnnouncementImageFile(imageFile('a.png'))
    expect(uri).toMatch(/^data:image\/png;base64,[A-Za-z0-9+/]+={0,2}$/)
  })

  test('throws unsupported_type for non-whitelisted extensions', async () => {
    await expect(encodeAnnouncementImageFile(imageFile('a.svg'))).rejects.toMatchObject({
      reason: 'unsupported_type',
    })
  })

  test('throws too_large above the gateway cap', async () => {
    await expect(
      encodeAnnouncementImageFile(imageFile('big.png', MAX_ANNOUNCEMENT_IMAGE_BYTES + 1))
    ).rejects.toMatchObject({ reason: 'too_large' })
  })

  test('cap matches the gateway limit (5 MiB)', () => {
    expect(MAX_ANNOUNCEMENT_IMAGE_BYTES).toBe(5 * 1024 * 1024)
  })
})
