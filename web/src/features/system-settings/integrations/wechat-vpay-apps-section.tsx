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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FlaskConical, Pencil, Plus, ShieldAlert, Trash2 } from 'lucide-react'
import * as React from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { EmptyState } from '@/components/empty-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { handleServerError } from '@/lib/handle-server-error'

import {
  createWechatVpayApp,
  deleteWechatVpayApp,
  listWechatVpayApps,
  probeWechatVpayGoods,
  updateWechatVpayApp,
  type WechatVpayAppView,
  type WechatVpayGoodsProbeResult,
} from './wechat-vpay-apps-api'

// 微信虚拟支付小程序管理：多小程序并存，凭证按 appid 维度独立。配置即时保存
// （独立于本页其余 option 表单的统一保存按钮）。

const WECHAT_VPAY_APPS_QUERY_KEY = ['wechat-vpay-apps'] as const

function getAppFormSchema(t: (key: string) => string) {
  return z.object({
    app_id: z.string().trim().min(1, t('Please enter the AppID')),
    name: z.string(),
    offer_id: z.string(),
    env: z.coerce.number().int().min(0).max(1),
    enabled: z.boolean(),
    // 密钥留空 = 保留原值（后端不回显明文）
    app_secret: z.string(),
    app_key: z.string(),
    sandbox_app_key: z.string(),
    push_token: z.string(),
  })
}

type AppFormValues = z.infer<ReturnType<typeof getAppFormSchema>>

function appFormDefaults(app?: WechatVpayAppView | null): AppFormValues {
  return {
    app_id: app?.app_id ?? '',
    name: app?.name ?? '',
    offer_id: app?.offer_id ?? '',
    env: app?.env ?? 0,
    enabled: app?.enabled ?? true,
    app_secret: '',
    app_key: '',
    sandbox_app_key: '',
    push_token: '',
  }
}

function appPayloadFromValues(values: AppFormValues) {
  return {
    app_id: values.app_id.trim(),
    name: values.name.trim(),
    offer_id: values.offer_id.trim(),
    env: values.env,
    enabled: values.enabled,
    app_secret: values.app_secret.trim(),
    app_key: values.app_key.trim(),
    sandbox_app_key: values.sandbox_app_key.trim(),
    push_token: values.push_token.trim(),
  }
}

// 编辑/新增弹窗。编辑时已配置的密钥以 has_* 提示，输入非空即覆盖。
function WechatVpayAppFormDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  editing: WechatVpayAppView | null
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const editing = props.editing
  const isEdit = !!editing
  const schema = getAppFormSchema(t)
  const form = useForm<AppFormValues>({
    resolver: zodResolver(schema) as Resolver<AppFormValues>,
    defaultValues: appFormDefaults(props.editing),
  })

  React.useEffect(() => {
    if (props.open) {
      form.reset(appFormDefaults(editing))
    }
  }, [props.open, editing, form])

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: WECHAT_VPAY_APPS_QUERY_KEY })

  const onSubmit = async (values: AppFormValues) => {
    const payload = appPayloadFromValues(values)
    try {
      const res = isEdit
        ? await updateWechatVpayApp(editing.id, payload)
        : await createWechatVpayApp(payload)
      if (!res.success) {
        handleServerError(res, t('Failed to save mini-program'))
        return
      }
      toast.success(t('Mini-program saved'))
      props.onOpenChange(false)
      invalidate()
    } catch (error) {
      handleServerError(error, t('Failed to save mini-program'))
    }
  }

  const secretPlaceholder = (configured: boolean) =>
    configured
      ? t('Configured — enter a new value to replace it')
      : t('Enter new secret to update')


  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        isEdit
          ? t('Edit mini-program: {{appId}}', { appId: editing.app_id })
          : t('Add mini-program')
      }
      description={t(
        'Credentials are per mini-program (MP console 虚拟支付 → 基本配置 / 消息推送). Secrets are write-only: leave blank to keep the stored value.'
      )}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={form.formState.isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => form.handleSubmit(onSubmit)()}
            disabled={form.formState.isSubmitting}
          >
            {form.formState.isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          className='space-y-4'
          onSubmit={(event) => {
            event.preventDefault()
            form.handleSubmit(onSubmit)(event)
          }}
        >
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='app_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Mini-program AppID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='wx1234567890abcdef'
                      autoComplete='off'
                      disabled={isEdit}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  {isEdit && (
                    <FormDescription>
                      {t(
                        'AppID cannot be changed once orders exist; delete and recreate instead.'
                      )}
                    </FormDescription>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Display name')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g., Qianmiao Mini-Program')}
                      autoComplete='off'
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='offer_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Offer ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='12345'
                      autoComplete='off'
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Found in MP console: 虚拟支付 → 基本配置')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='env'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Environment')}</FormLabel>
                  <Select
                    value={String(field.value)}
                    onValueChange={(value) => field.onChange(Number(value))}
                  >
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='0'>
                        {t('0 = production (personal subject)')}
                      </SelectItem>
                      <SelectItem value='1'>{t('1 = sandbox')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                <div className='space-y-0.5'>
                  <FormLabel>{t('Enabled')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Disabled mini-programs cannot create orders and their push endpoints are rejected.'
                    )}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='app_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Mini-program AppSecret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={secretPlaceholder(
                        !!props.editing?.has_app_secret
                      )}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Only needed for the query_order reconciliation endpoint; leave blank unless rotating'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='app_key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Production AppKey')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={secretPlaceholder(
                        !!props.editing?.has_app_key
                      )}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='sandbox_app_key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Sandbox AppKey')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={secretPlaceholder(
                        !!props.editing?.has_sandbox_app_key
                      )}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Only used when Env is set to 1 (sandbox)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='push_token'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Message Push Token')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={secretPlaceholder(
                        !!props.editing?.has_push_token
                      )}
                      {...field}
                      onChange={(event) =>
                        field.onChange(event.target.value)
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Configured with the push URL in this mini-program’s MP console; verifies every push'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </form>
      </Form>
    </Dialog>
  )
}

// 道具接口权限探测：上传一个开发版本测试道具（probe_test_item）到
// /xpay/start_upload_goods，拿 errcode 判定该小程序能否调用道具 API——
// 这是「套餐自动推送道具」可行性的前置实测。
function WechatVpayGoodsProbeDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  app: WechatVpayAppView | null
}) {
  const { t } = useTranslation()
  const [itemUrl, setItemUrl] = React.useState('')
  const [probing, setProbing] = React.useState(false)
  const [result, setResult] = React.useState<WechatVpayGoodsProbeResult | null>(
    null
  )

  React.useEffect(() => {
    if (props.open) {
      setItemUrl('')
      setResult(null)
    }
  }, [props.open])

  const runProbe = async () => {
    if (!props.app) return
    setProbing(true)
    try {
      const res = await probeWechatVpayGoods(props.app.id, itemUrl.trim())
      if (!res.success || !res.data) {
        handleServerError(res, t('Probe failed'))
        return
      }
      setResult(res.data)
    } catch (error) {
      handleServerError(error, t('Probe failed'))
    } finally {
      setProbing(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Probe WeChat goods API access')}
      description={t(
        'Uploads one throwaway dev-version test item (probe_test_item, 1 fen) to /xpay/start_upload_goods. Any xpay business errcode means the API is reachable; 48001 means this mini-program is not authorized (personal-subject apps likely cannot call the goods APIs). The test item never gets published and can be deleted in the MP console.'
      )}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Close')}
          </Button>
          <Button
            type='button'
            onClick={runProbe}
            disabled={probing || !props.app?.has_app_secret}
          >
            {probing ? t('Probing...') : t('Run probe')}
          </Button>
        </>
      }
    >
      <div className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='wechat-vpay-probe-item-url'>
            {t('Item image URL (jpg/png, publicly reachable)')}
          </Label>
          <Input
            id='wechat-vpay-probe-item-url'
            placeholder={t('Leave blank to use <ServerAddress>/logo.png')}
            value={itemUrl}
            onChange={(event) => setItemUrl(event.target.value)}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'WeChat fetches and validates this image; an unreachable URL is rejected with errcode 268490002.'
            )}
          </p>
        </div>
        {props.app && !props.app.has_app_secret && (
          <Alert variant='destructive'>
            <ShieldAlert className='h-4 w-4' />
            <AlertTitle>{t('AppSecret missing')}</AlertTitle>
            <AlertDescription>
              {t('The probe needs the AppSecret to obtain an access_token.')}
            </AlertDescription>
          </Alert>
        )}
        {result && (
          <div className='space-y-2 rounded-lg border p-3 text-sm'>
            <div>
              errcode: <span className='font-mono'>{result.errcode}</span>
              {result.errmsg && (
                <span className='text-muted-foreground'>
                  {' '}
                  ({result.errmsg})
                </span>
              )}
            </div>
            <p className='whitespace-pre-wrap'>{result.verdict}</p>
          </div>
        )}
      </div>
    </Dialog>
  )
}

export function WechatVpayAppsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const appsQuery = useQuery({
    queryKey: WECHAT_VPAY_APPS_QUERY_KEY,
    queryFn: listWechatVpayApps,
  })
  const apps =
    appsQuery.data && appsQuery.data.success
      ? appsQuery.data.data ?? []
      : []

  const [dialogOpen, setDialogOpen] = React.useState(false)
  const [editing, setEditing] = React.useState<WechatVpayAppView | null>(null)
  const [deleting, setDeleting] = React.useState<WechatVpayAppView | null>(
    null
  )
  const [probing, setProbing] = React.useState<WechatVpayAppView | null>(null)

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteWechatVpayApp(id),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(t('Mini-program removed'))
        setDeleting(null)
        queryClient.invalidateQueries({ queryKey: WECHAT_VPAY_APPS_QUERY_KEY })
        return
      }
      handleServerError(res, t('Failed to remove mini-program'))
    },
  })

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>
          {t('WeChat Mini-Program Virtual Payment')}
        </h3>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Personal-subject 道具直购 (short_series_goods) integration. Multiple mini-programs are supported: each one gets its own credentials, registered items, and push URL. Plans map to a per-app item in Subscription plans.'
          )}
        </p>
      </div>

      <Alert>
        <ShieldAlert className='h-4 w-4' />
        <AlertTitle>{t('Push Configuration:')}</AlertTitle>
        <AlertDescription>
          {t(
            'Configure each mini-program’s MP console under 开发管理 → 消息推送 (plaintext mode) with the URL below and the Token saved here. The URL routes by AppID:'
          )}{' '}
          <code className='rounded bg-muted px-1 py-0.5 text-xs'>
            {'<ServerAddress>/api/subscription/wechat-vpay/notify/<AppID>'}
          </code>
        </AlertDescription>
      </Alert>

      <div className='flex justify-end'>
        <Button
          type='button'
          size='sm'
          onClick={() => {
            setEditing(null)
            setDialogOpen(true)
          }}
        >
          <Plus className='mr-1 h-4 w-4' />
          {t('Add mini-program')}
        </Button>
      </div>

      {apps.length === 0 ? (
        <EmptyState
          title={t('No mini-programs configured')}
          description={t(
            'Add each mini-program that uses WeChat virtual payment. Its AppID/OfferID/AppKey come from the MP console 虚拟支付 page.'
          )}
        />
      ) : (
        <div className='space-y-3'>
          {apps.map((app) => (
            <div
              key={app.id}
              className='flex flex-col gap-3 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between'
            >
              <div className='min-w-0'>
                <div className='flex flex-wrap items-center gap-2'>
                  <span className='font-medium'>
                    {app.name || app.app_id}
                  </span>
                  <code className='text-muted-foreground rounded bg-muted px-1 py-0.5 text-xs'>
                    {app.app_id}
                  </code>
                  {app.enabled ? (
                    <Badge>{t('Enabled')}</Badge>
                  ) : (
                    <Badge variant='outline'>{t('Disabled')}</Badge>
                  )}
                  {app.signing_ready ? (
                    <Badge variant='secondary'>
                      {t('Order signing ready')}
                    </Badge>
                  ) : (
                    <Badge variant='destructive'>
                      {t('Signing incomplete')}
                    </Badge>
                  )}
                  {app.enabled &&
                    (app.push_ready ? (
                      <Badge variant='secondary'>{t('Push ready')}</Badge>
                    ) : (
                      <Badge variant='destructive'>
                        {t('Push token missing')}
                      </Badge>
                    ))}
                </div>
                <p className='text-muted-foreground mt-1 text-xs'>
                  {t('Offer ID: {{offerId}} · Env: {{env}}', {
                    offerId: app.offer_id || '-',
                    env: app.env,
                  })}
                </p>
              </div>
              <div className='flex shrink-0 gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => {
                    setEditing(app)
                    setDialogOpen(true)
                  }}
                >
                  <Pencil className='mr-1 h-3.5 w-3.5' />
                  {t('Edit')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setProbing(app)}
                  title={t('Probe WeChat goods API access')}
                >
                  <FlaskConical className='mr-1 h-3.5 w-3.5' />
                  {t('Probe')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  className='text-destructive'
                  onClick={() => setDeleting(app)}
                >
                  <Trash2 className='mr-1 h-3.5 w-3.5' />
                  {t('Delete')}
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <WechatVpayAppFormDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        editing={editing}
      />

      <WechatVpayGoodsProbeDialog
        open={!!probing}
        onOpenChange={(open) => {
          if (!open) {
            setProbing(null)
          }
        }}
        app={probing}
      />

      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(open) => {
          if (!open) {
            setDeleting(null)
          }
        }}
        title={t('Remove mini-program')}
        desc={t(
          'Remove {{appId}}? Existing orders keep their history, but pushes and query_order reconciliation for it stop working.',
          { appId: deleting?.app_id ?? '' }
        )}
        destructive
        confirmText={t('Remove')}
        isLoading={deleteMutation.isPending}
        disabled={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleting) {
            deleteMutation.mutate(deleting.id)
          }
        }}
      />
    </div>
  )
}
