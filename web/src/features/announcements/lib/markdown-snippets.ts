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
import { getPreviewText } from '@/features/dashboard/lib/text'

/**
 * 在 textarea 选区处插入片段，返回新文本与插入后光标位置。
 * 块级片段（图片/独占行）自动补换行保证独占一行；行内包裹保持紧贴选区。
 */
export function insertSnippet(
  value: string,
  start: number,
  end: number,
  snippet: string,
  options: { block?: boolean } = {}
): { value: string; cursor: number } {
  const block = options.block ?? false
  let text = snippet
  let prefix = ''
  if (block) {
    if (start > 0 && value[start - 1] !== '\n') prefix = '\n'
    if (start < end || start < value.length) {
      if (value[end] !== undefined && value[end] !== '\n') text += '\n'
    }
  }
  const next =
    value.slice(0, start) + prefix + text + value.slice(end)
  const cursor = start + prefix.length + snippet.length
  return { value: next, cursor }
}

/**
 * 用前后缀包裹选中文本（如 **加粗**）。无选中时插入前后缀并把光标放中间。
 * 若该行已有此前缀则原样返回（重复点击不叠加）。
 */
export function wrapSelection(
  value: string,
  start: number,
  end: number,
  before: string,
  after = before
): { value: string; cursorStart: number; cursorEnd: number } {
  const selected = value.slice(start, end)
  if (selected.startsWith(before) && selected.endsWith(after)) {
    return { value, cursorStart: start, cursorEnd: end }
  }
  const next = value.slice(0, start) + before + selected + after + value.slice(end)
  if (selected.length === 0) {
    return { value: next, cursorStart: start + before.length, cursorEnd: start + before.length }
  }
  return {
    value: next,
    cursorStart: start + before.length,
    cursorEnd: start + before.length + selected.length,
  }
}

/**
 * 在光标所在行行首插入前缀（标题 #、列表 - 等）。
 * 空选区时对光标所在行生效；有选区时对选区覆盖的每一行生效。
 */
export function prefixLines(
  value: string,
  start: number,
  end: number,
  prefix: string
): { value: string; cursorStart: number; cursorEnd: number } {
  const lineStart = value.lastIndexOf('\n', start - 1) + 1
  let lineEnd = value.indexOf('\n', end)
  if (lineEnd === -1) lineEnd = value.length
  const lines = value.slice(lineStart, lineEnd).split('\n')
  const prefixed = lines.map((line) => (line.startsWith(prefix) ? line : prefix + line))
  const next = value.slice(0, lineStart) + prefixed.join('\n') + value.slice(lineEnd)
  return { value: next, cursorStart: lineStart, cursorEnd: lineStart + prefixed.join('\n').length }
}

/**
 * 列表副标题预览：先剥掉 Markdown 图片/链接语法（保留链接文字），
 * 再交给通用 getPreviewText 去标记截断。
 */
export function announcementPreviewText(content: string, maxLength = 60): string {
  const withoutLinks = content.replaceAll(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
  return getPreviewText(withoutLinks, maxLength)
}
