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

import { insertSnippet, prefixLines, wrapSelection } from '../markdown-snippets'

describe('insertSnippet', () => {
  test('inline snippet replaces the selection without padding', () => {
    const r = insertSnippet('hello world', 6, 11, 'there')
    expect(r.value).toBe('hello there')
    expect(r.cursor).toBe('hello there'.length)
  })

  test('inline snippet in an empty editor keeps the text intact', () => {
    const r = insertSnippet('', 0, 0, 'first line')
    expect(r.value).toBe('first line')
    expect(r.cursor).toBe('first line'.length)
  })

  test('block snippet lands on its own line when the cursor is mid-line', () => {
    const r = insertSnippet('para one', 8, 8, '![](img.png)', { block: true })
    expect(r.value).toBe('para one\n![](img.png)')
    expect(r.cursor).toBe('para one\n'.length + '![](img.png)'.length)
  })

  test('block snippet adds a trailing newline when followed by text', () => {
    const r = insertSnippet('abXcd', 2, 3, '![](img.png)', { block: true })
    expect(r.value).toBe('ab\n![](img.png)\ncd')
  })

  test('block snippet does not double newlines at document edges', () => {
    const atStart = insertSnippet('first', 0, 0, '![](img.png)', { block: true })
    expect(atStart.value).toBe('![](img.png)\nfirst')
    const atEnd = insertSnippet('last\n', 5, 5, '![](img.png)', { block: true })
    expect(atEnd.value).toBe('last\n![](img.png)')
  })
})

describe('wrapSelection', () => {
  test('wraps the selected text with the marker', () => {
    const r = wrapSelection('say bold now', 4, 8, '**')
    expect(r.value).toBe('say **bold** now')
    expect(r.cursorStart).toBe(6)
    expect(r.cursorEnd).toBe(10)
  })

  test('inserts an empty marker pair when nothing is selected', () => {
    const r = wrapSelection('say  now', 4, 4, '**')
    expect(r.value).toBe('say **** now')
    expect(r.cursorStart).toBe(6)
    expect(r.cursorEnd).toBe(6)
  })

  test('does not stack markers on an already-wrapped selection', () => {
    const r = wrapSelection('**bold**', 0, 8, '**')
    expect(r.value).toBe('**bold**')
    expect(r.cursorStart).toBe(0)
    expect(r.cursorEnd).toBe(8)
  })
})

describe('prefixLines', () => {
  test('prefixes the line holding an empty selection', () => {
    const r = prefixLines('one\ntwo\nthree', 4, 4, '## ')
    expect(r.value).toBe('one\n## two\nthree')
    expect(r.cursorStart).toBe(4)
    expect(r.cursorEnd).toBe('## two'.length + 4)
  })

  test('prefixes every line covered by the selection', () => {
    const r = prefixLines('a\nb\nc', 0, 3, '- ')
    expect(r.value).toBe('- a\n- b\nc')
  })

  test('does not add the prefix twice', () => {
    const r = prefixLines('## head', 0, 7, '## ')
    expect(r.value).toBe('## head')
  })

  test('handles the last line without a trailing newline', () => {
    const r = prefixLines('x\ny', 2, 2, '- ')
    expect(r.value).toBe('x\n- y')
  })
})
