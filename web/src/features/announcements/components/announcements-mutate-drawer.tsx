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
import { zodResolver } from '@hookform/resolvers/zod'
import { Megaphone } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { handleServerError } from '@/lib/handle-server-error'
import {
  formatTimestampForInput,
  parseTimestampFromInput,
} from '@/lib/format'

import { createAnnouncement, updateAnnouncement } from '../api'
import { ANNOUNCEMENT_APPS, type Announcement } from '../types'
import { useAnnouncements } from './announcements-provider'

const ANNOUNCEMENT_TYPES = [
  'default',
  'ongoing',
  'success',
  'warning',
  'error',
] as const

const TYPE_LABEL_KEYS: Record<(typeof ANNOUNCEMENT_TYPES)[number], string> = {
  default: 'Default',
  ongoing: 'Ongoing',
  success: 'Success',
  warning: 'Warning',
  error: 'Error',
}

type AnnouncementFormValues = z.infer<ReturnType<typeof getFormSchema>>

function getFormSchema(t: (key: string) => string) {
  return z.object({
    app: z.string().min(1, t('App is required')),
    title: z.string().min(1, t('Title is required')),
    type: z.enum(ANNOUNCEMENT_TYPES),
    publish_time: z.string(), // datetime-local value; '' = publish immediately
    link_url: z
      .string()
      .refine(
        (v) => v === '' || /^https?:\/\//.test(v),
        t('Link must start with http:// or https://')
      ),
    content: z.string(),
    enabled: z.boolean(),
  })
}

const FORM_DEFAULTS: AnnouncementFormValues = {
  app: ANNOUNCEMENT_APPS[0].value,
  title: '',
  type: 'default',
  publish_time: '',
  link_url: '',
  content: '',
  enabled: true,
}

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Announcement
}

export function AnnouncementsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: Props) {
  const { t } = useTranslation()
  const isEdit = !!currentRow?.id
  const { triggerRefresh } = useAnnouncements()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const schema = getFormSchema(t)
  const form = useForm<AnnouncementFormValues>({
    resolver: zodResolver(schema) as unknown as Resolver<AnnouncementFormValues>,
    defaultValues: FORM_DEFAULTS,
  })

  useEffect(() => {
    if (open) {
      if (currentRow) {
        form.reset({
          app: currentRow.app,
          title: currentRow.title,
          type: ANNOUNCEMENT_TYPES.includes(currentRow.type)
            ? currentRow.type
            : 'default',
          publish_time: formatTimestampForInput(currentRow.publish_time),
          link_url: currentRow.link_url || '',
          content: currentRow.content,
          enabled: currentRow.enabled,
        })
      } else {
        form.reset(FORM_DEFAULTS)
      }
    }
  }, [open, currentRow, form])

  const onSubmit = async (values: AnnouncementFormValues) => {
    setIsSubmitting(true)
    try {
      // 空时间 = 立即发布（后端 0 → 当前时间）；编辑时清空即改为立即发布
      const parsed = values.publish_time
        ? parseTimestampFromInput(values.publish_time)
        : 0
      const payload = {
        app: values.app,
        title: values.title.trim(),
        content: values.content,
        link_url: values.link_url.trim(),
        type: values.type,
        publish_time: parsed > 0 ? parsed : 0,
        enabled: values.enabled,
      }
      if (isEdit && currentRow?.id) {
        const res = await updateAnnouncement(currentRow.id, payload)
        if (res.success) {
          toast.success(t('Update succeeded'))
          onOpenChange(false)
          triggerRefresh()
        } else {
          handleServerError(res)
        }
      } else {
        const res = await createAnnouncement(payload)
        if (res.success) {
          toast.success(t('Create succeeded'))
          onOpenChange(false)
          triggerRefresh()
        } else {
          handleServerError(res)
        }
      }
    } catch (error) {
      handleServerError(error, t('Request failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[520px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEdit ? t('Update announcement') : t('Create Announcement')}
          </SheetTitle>
          <SheetDescription>
            {isEdit
              ? t('Modify the announcement delivered to the app')
              : t('Create a new announcement for external apps')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='announcement-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <h3 className='flex items-center gap-2 text-sm font-medium'>
                <IconBadge tone='info' size='xs'>
                  <Megaphone />
                </IconBadge>
                {t('Basic Info')}
              </h3>

              <FormField
                control={form.control}
                name='app'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('App')}</FormLabel>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={isEdit}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {ANNOUNCEMENT_APPS.map((app) => (
                            <SelectItem key={app.value} value={app.value}>
                              {app.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='title'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Title')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('e.g. New version released')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Type')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectGroup>
                          {ANNOUNCEMENT_TYPES.map((typ) => (
                            <SelectItem key={typ} value={typ}>
                              {t(TYPE_LABEL_KEYS[typ])}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='publish_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Publish Date')}</FormLabel>
                    <FormControl>
                      <Input type='datetime-local' {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('Leave empty to publish immediately')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='link_url'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Link URL')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder='https://mp.weixin.qq.com/s/...'
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'If set, tapping the announcement opens this URL directly; official account articles require the account to be associated with the mini-program'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <h3 className='flex items-center gap-2 text-sm font-medium'>
                <IconBadge tone='info' size='xs'>
                  <Megaphone />
                </IconBadge>
                {t('Content')}
              </h3>

              <FormField
                control={form.control}
                name='content'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Content')}</FormLabel>
                    <FormControl>
                      <Textarea rows={8} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t('Plain text; line breaks are preserved')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem>
                    <div className={sideDrawerSwitchItemClassName()}>
                      <FormLabel>{t('Enable')}</FormLabel>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </div>
                    <FormDescription>
                      {t(
                        'Disabled announcements are hidden from the app but kept in the list'
                      )}
                    </FormDescription>
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='announcement-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
