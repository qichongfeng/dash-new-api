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
  transformChannelToFormDefaults,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from '../channel-form'

const VISION_PROTOCOLS_JSON =
  '{"@cf/meta/llama-3.2-11b-vision-instruct":"llama-vision"}'

function formValues(overrides: Partial<ChannelFormValues>): ChannelFormValues {
  return {
    ...{ type: 39, settings: '{}', vision_protocols: '', group: ['default'], models: '' },
    ...overrides,
  } as ChannelFormValues
}

describe('vision_protocols setting round-trip', () => {
  test('type 39 form value is merged into the submitted settings JSON', () => {
    const payload = transformFormDataToUpdatePayload(
      formValues({ vision_protocols: VISION_PROTOCOLS_JSON }),
      1
    )
    const parsed = JSON.parse(payload.settings!)
    expect(parsed.vision_protocols).toEqual({
      '@cf/meta/llama-3.2-11b-vision-instruct': 'llama-vision',
    })
  })

  test('existing settings keys are preserved when writing vision_protocols', () => {
    const payload = transformFormDataToUpdatePayload(
      formValues({
        settings: '{"custom_key":"keep-me"}',
        vision_protocols: VISION_PROTOCOLS_JSON,
      }),
      1
    )
    const parsed = JSON.parse(payload.settings!)
    expect(parsed.custom_key).toBe('keep-me')
    expect(parsed.vision_protocols).toBeDefined()
  })

  test('non-Cloudflare channels drop the key from settings', () => {
    const payload = transformFormDataToUpdatePayload(
      formValues({
        type: 1,
        settings: `{"vision_protocols":${VISION_PROTOCOLS_JSON}}`,
      }),
      1
    )
    expect(JSON.parse(payload.settings!).vision_protocols).toBeUndefined()
  })

  test('editing a channel hydrates the form field from settings JSON', () => {
    const defaults = transformChannelToFormDefaults({
      channel_info: {},
      settings: `{"vision_protocols":${VISION_PROTOCOLS_JSON}}`,
    } as Parameters<typeof transformChannelToFormDefaults>[0])
    expect(defaults.vision_protocols).toBe(VISION_PROTOCOLS_JSON)
  })
})
