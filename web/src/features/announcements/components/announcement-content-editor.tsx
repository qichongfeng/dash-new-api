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
import {
  Bold,
  Heading2,
  ImagePlus,
  Italic,
  Link2,
  List,
  Loader2,
} from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'

import { uploadAnnouncementImage } from '../api'
import {
  AnnouncementImageFileError,
  encodeAnnouncementImageFile,
} from '../lib/announcement-image'
import { insertSnippet, prefixLines, wrapSelection } from '../lib/markdown-snippets'

interface Props {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}

type EditorMode = 'write' | 'preview'

/**
 * 公告正文 Markdown 编辑器：工具栏（格式/链接/图片上传）+ 编辑/预览切换。
 * 预览走共享 Markdown 渲染器（消毒），与小程序端 marked 渲染语义一致（breaks、gfm）。
 */
export function AnnouncementContentEditor({ value, onChange, disabled }: Props) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<EditorMode>('write')
  const [uploading, setUploading] = useState(false)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  // 待恢复的光标位置：插入片段重渲染后统一在 effect 里落回 textarea
  const pendingCursor = useRef<{ start: number; end: number } | null>(null)

  useEffect(() => {
    if (pendingCursor.current && textareaRef.current) {
      const { start, end } = pendingCursor.current
      pendingCursor.current = null
      textareaRef.current.focus()
      textareaRef.current.setSelectionRange(start, end)
    }
  }, [value])

  // 统一入口：对当前选区应用纯函数变换，写回表单并安排光标恢复
  const applyTransform = (
    transform: (value: string, start: number, end: number) => {
      value: string
      cursorStart: number
      cursorEnd: number
    }
  ) => {
    const textarea = textareaRef.current
    const start = textarea?.selectionStart ?? value.length
    const end = textarea?.selectionEnd ?? value.length
    const result = transform(value, start, end)
    onChange(result.value)
    pendingCursor.current = { start: result.cursorStart, end: result.cursorEnd }
  }

  const handleWrap = (marker: string) => {
    applyTransform((v, s, e) => {
      const r = wrapSelection(v, s, e, marker)
      return { value: r.value, cursorStart: r.cursorStart, cursorEnd: r.cursorEnd }
    })
  }

  const handleLinePrefix = (prefix: string) => {
    applyTransform((v, s, e) => {
      const r = prefixLines(v, s, e, prefix)
      return { value: r.value, cursorStart: r.cursorStart, cursorEnd: r.cursorEnd }
    })
  }

  const handleLink = () => {
    applyTransform((v, s, e) => {
      const selected = v.slice(s, e)
      const snippet = `[${selected}](https://)`
      const r = insertSnippet(v, s, e, snippet)
      return { value: r.value, cursorStart: r.cursor, cursorEnd: r.cursor }
    })
  }

  const handleImageFile = async (file: File | undefined) => {
    if (!file) return
    let dataUri: string
    try {
      dataUri = await encodeAnnouncementImageFile(file)
    } catch (error) {
      if (error instanceof AnnouncementImageFileError) {
        toast.error(
          error.reason === 'too_large'
            ? t('Image exceeds the 5 MiB limit.')
            : t('Image must be a PNG, JPG, GIF or WebP file.')
        )
      } else {
        toast.error(t('Image upload failed'))
      }
      return
    }
    setUploading(true)
    try {
      const res = await uploadAnnouncementImage(dataUri)
      const src = res.data?.src
      if (!res.success || !src) {
        handleServerError(res, t('Image upload failed'))
        return
      }
      const markdown = `![](${src})`
      applyTransform((v, s, e) => {
        const r = insertSnippet(v, s, e, markdown, { block: true })
        return { value: r.value, cursorStart: r.cursor, cursorEnd: r.cursor }
      })
    } catch (error) {
      handleServerError(error, t('Image upload failed'))
    } finally {
      setUploading(false)
    }
  }

  const toolbarButton = (
    label: string,
    icon: ReactNode,
    onClick: () => void
  ) => (
    <Button
      type='button'
      variant='ghost'
      size='sm'
      onClick={onClick}
      disabled={disabled || uploading || mode === 'preview'}
      aria-label={label}
      title={label}
    >
      {icon}
    </Button>
  )

  return (
    <div className='space-y-2'>
      <div className='flex items-center gap-0.5 rounded-lg border p-0.5'>
        {toolbarButton(t('Bold'), <Bold />, () => handleWrap('**'))}
        {toolbarButton(t('Italic'), <Italic />, () => handleWrap('*'))}
        {toolbarButton(t('Heading'), <Heading2 />, () => handleLinePrefix('## '))}
        {toolbarButton(t('List'), <List />, () => handleLinePrefix('- '))}
        {toolbarButton(t('Link'), <Link2 />, handleLink)}
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => fileInputRef.current?.click()}
          disabled={disabled || uploading || mode === 'preview'}
          aria-label={t('Insert Image')}
          title={t('Insert Image')}
        >
          {uploading ? <Loader2 className='animate-spin' /> : <ImagePlus />}
        </Button>
        <input
          ref={fileInputRef}
          type='file'
          accept='.png,.jpg,.jpeg,.gif,.webp,image/png,image/jpeg,image/gif,image/webp'
          className='hidden'
          onChange={(e) => {
            void handleImageFile(e.target.files?.[0])
            e.target.value = ''
          }}
        />
        <div className='ml-auto flex items-center gap-0.5'>
          <Button
            type='button'
            variant={mode === 'write' ? 'secondary' : 'ghost'}
            size='sm'
            onClick={() => setMode('write')}
            disabled={disabled}
          >
            {t('Write')}
          </Button>
          <Button
            type='button'
            variant={mode === 'preview' ? 'secondary' : 'ghost'}
            size='sm'
            onClick={() => setMode('preview')}
            disabled={disabled}
          >
            {t('Preview')}
          </Button>
        </div>
      </div>
      {mode === 'write' ? (
        <Textarea
          ref={textareaRef}
          rows={10}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          aria-label={t('Content')}
        />
      ) : (
        <div className='min-h-40 rounded-lg border p-3'>
          {value.trim() ? (
            <Markdown breaks className='text-sm'>
              {value}
            </Markdown>
          ) : (
            <p className='text-muted-foreground text-sm'>{t('No content')}</p>
          )}
        </div>
      )}
    </div>
  )
}
