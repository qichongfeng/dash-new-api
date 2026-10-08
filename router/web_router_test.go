package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWebPageGateEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetWebRouter(engine, WebAssets{IndexPage: []byte("<html>spa</html>")}, func(c *gin.Context) { c.Next() })
	return engine
}

func doWebPageRequest(engine *gin.Engine, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	engine.ServeHTTP(w, req)
	return w
}

func TestWebPageGateBlocksAnonymousPages(t *testing.T) {
	engine := newWebPageGateEngine(t)
	for _, target := range []string{"/", "/dashboard", "/security", "/otp", "/user-agreement", "/privacy-policy"} {
		w := doWebPageRequest(engine, target)
		assert.Equal(t, http.StatusForbidden, w.Code, target)
		assert.Contains(t, w.Header().Get("Cache-Control"), "no-store", target)
		assert.Contains(t, w.Body.String(), `"success":false`, target)
	}
}

func TestWebPageGateAllowsSignIn(t *testing.T) {
	engine := newWebPageGateEngine(t)
	w := doWebPageRequest(engine, "/sign-in")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "<html>spa</html>", w.Body.String())
}

func TestWebPageGateSetupOnlyBeforeInit(t *testing.T) {
	engine := newWebPageGateEngine(t)
	saved := constant.Setup
	t.Cleanup(func() { constant.Setup = saved })

	constant.Setup = false
	w := doWebPageRequest(engine, "/setup")
	require.Equal(t, http.StatusOK, w.Code)

	constant.Setup = true
	w = doWebPageRequest(engine, "/setup")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWebPageGateSessionHintPasses(t *testing.T) {
	engine := newWebPageGateEngine(t)
	w := doWebPageRequest(engine, "/dashboard", &http.Cookie{Name: service.SessionHintCookieName, Value: "1"})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "<html>spa</html>", w.Body.String())
}

func TestWebPageGateStaticAssetsPass(t *testing.T) {
	engine := newWebPageGateEngine(t)
	for _, target := range []string{"/favicon.ico", "/logo.png", "/static/js/app.js"} {
		w := doWebPageRequest(engine, target)
		assert.NotEqual(t, http.StatusForbidden, w.Code, target)
	}
}

func TestWebPageGateKeepsRelayNotFoundForAPIProbes(t *testing.T) {
	engine := newWebPageGateEngine(t)
	w := doWebPageRequest(engine, "/api/nonexistent")
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`, "unmatched /api probe should keep RelayNotFound response")
}
