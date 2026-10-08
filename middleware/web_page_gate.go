package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// WebPageGate 隐藏面板 SPA：匿名访客只能看到 /sign-in（以及首次初始化前的
// /setup）和静态资源，其余页面路径一律 403 JSON，不向陌生访客暴露面板形态。
// 已登录浏览器的放行信号是会话提示 cookie（与 Refresh Cookie 同写同清、自身
// 无凭证语义）：真正的访问控制仍由 SPA 路由守卫与 /api 鉴权执行，伪造提示
// cookie 最多换回与 /sign-in 相同的 index.html。
func WebPageGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case path == "/sign-in":
			c.Next()
			return
		case path == "/setup" && !constant.Setup:
			c.Next()
			return
		case strings.HasPrefix(path, "/api"), strings.HasPrefix(path, "/v1"):
			// 未匹配到的 /api、/v1 探测维持原有 RelayNotFound 兜底。
			c.Next()
			return
		case strings.Contains(path[strings.LastIndex(path, "/")+1:], "."):
			// 静态资源形态（favicon、logo、/static/**），登录页渲染依赖它们。
			c.Next()
			return
		}
		if hint, err := c.Request.Cookie(service.SessionHintCookieName); err == nil && hint.Value != "" {
			c.Next()
			return
		}
		c.Header("Cache-Control", "no-store")
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"code":    "WEB_PAGE_FORBIDDEN",
			"message": "forbidden",
		})
	}
}
