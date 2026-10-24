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

// 微信虚拟支付多小程序管理接口。凭证只写不读：后端返回 has_* 布尔而非明文，
// 更新时密钥留空 = 保留原值。

export interface WechatVpayAppView {
  id: number
  app_id: string
  name: string
  offer_id: string
  env: number
  enabled: boolean
  has_app_secret: boolean
  has_app_key: boolean
  has_sandbox_app_key: boolean
  has_push_token: boolean
  signing_ready: boolean
  push_ready: boolean
  created_at: number
  updated_at: number
}

export interface WechatVpayAppPayload {
  app_id: string
  name: string
  app_secret: string
  offer_id: string
  app_key: string
  sandbox_app_key: string
  push_token: string
  env: number
  enabled?: boolean
}

interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export async function listWechatVpayApps(): Promise<
  ApiResponse<WechatVpayAppView[]>
> {
  const res = await api.get<ApiResponse<WechatVpayAppView[]>>(
    '/api/subscription/admin/wechat-vpay/apps'
  )
  return res.data
}

export async function createWechatVpayApp(
  payload: WechatVpayAppPayload
): Promise<ApiResponse<WechatVpayAppView>> {
  const res = await api.post<ApiResponse<WechatVpayAppView>>(
    '/api/subscription/admin/wechat-vpay/apps',
    payload
  )
  return res.data
}

export async function updateWechatVpayApp(
  id: number,
  payload: WechatVpayAppPayload
): Promise<ApiResponse<WechatVpayAppView>> {
  const res = await api.put<ApiResponse<WechatVpayAppView>>(
    `/api/subscription/admin/wechat-vpay/apps/${id}`,
    payload
  )
  return res.data
}

export async function deleteWechatVpayApp(
  id: number
): Promise<ApiResponse<null>> {
  const res = await api.delete<ApiResponse<null>>(
    `/api/subscription/admin/wechat-vpay/apps/${id}`
  )
  return res.data
}

export interface WechatVpayGoodsProbeResult {
  errcode: number
  errmsg: string
  verdict: string
  raw: string
}

// 一次性探测：该小程序能否调用 /xpay/start_upload_goods（个人主体文档未列
// 道具接口，能否调需实测）。上传的是开发版本测试道具，不发布、可删。
export async function probeWechatVpayGoods(
  id: number,
  itemUrl: string
): Promise<ApiResponse<WechatVpayGoodsProbeResult>> {
  const res = await api.post<ApiResponse<WechatVpayGoodsProbeResult>>(
    `/api/subscription/admin/wechat-vpay/apps/${id}/goods-probe`,
    { item_url: itemUrl }
  )
  return res.data
}
