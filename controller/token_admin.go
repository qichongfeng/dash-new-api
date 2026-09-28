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
package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// adminTokenTarget resolves and authorizes the target user of an admin token
// operation. It writes the error response and returns false on failure.
func adminTokenTarget(c *gin.Context, userId int) (*model.User, bool) {
	if userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return nil, false
	}
	target, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserNotExists)
		return nil, false
	}
	if target.Status != common.UserStatusEnabled {
		common.ApiErrorI18n(c, i18n.MsgAuthUserBanned)
		return nil, false
	}
	if !canManageTargetRole(c.GetInt("role"), target.Role) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return nil, false
	}
	return target, true
}

// AdminGetUserTokens lists another user's tokens with masked keys so an
// external system can discover a user's API keys with an admin PAT.
func AdminGetUserTokens(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}
	userId, err := strconv.Atoi(c.Query("user_id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	target, ok := adminTokenTarget(c, userId)
	if !ok {
		return
	}

	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(target.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountUserTokens(target.Id)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

// AdminGetUserTokenKey reveals the full key of another user's token for
// external system integration. Ownership is scoped to the requested user_id.
func AdminGetUserTokenKey(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userId, err := strconv.Atoi(c.Query("user_id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	target, ok := adminTokenTarget(c, userId)
	if !ok {
		return
	}

	token, err := model.GetTokenByIds(id, target.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params := tokenAuditParams(c)
	params["id"], params["name"] = token.Id, token.Name
	params["target_user_id"] = target.Id
	common.SetContextKey(c, constant.ContextKeyTokenAuditSucceeded, true)
	common.ApiSuccess(c, gin.H{
		"key": token.GetFullKey(),
	})
}

// adminTokenRequest couples the regular token payload with the target owner.
type adminTokenRequest struct {
	tokenRequest
	UserId int `json:"user_id"`
}

// AdminAddUserToken issues a new API token owned by the requested user_id so
// an external system can provision keys with an admin PAT alone. Creation
// policy (name/quota/count/auto-groups) is scoped to the target user.
func AdminAddUserToken(c *gin.Context) {
	if c.GetInt("role") < common.RoleAdminUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return
	}
	request := adminTokenRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	target, ok := adminTokenTarget(c, request.UserId)
	if !ok {
		return
	}
	ownerGroup, err := model.GetUserGroup(target.Id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	params := tokenAuditParams(c)
	params["target_user_id"] = target.Id
	token, ok := createTokenForOwner(c, target.Id, ownerGroup, request.tokenRequest, params)
	if !ok {
		return
	}
	common.SetContextKey(c, constant.ContextKeyTokenAuditSucceeded, true)
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}
