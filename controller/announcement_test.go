package controller

import (
	"bytes"
	"encoding/base64"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAnnouncementControllerTest(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Announcement{}))
	model.DB = database
	t.Cleanup(func() { model.DB = originalDB })
}

func announcementImageDataURI(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// 各格式最小魔数片段（http.DetectContentType 只看头部签名）
var (
	announcementPNGBytes  = []byte("\x89PNG\r\n\x1a\n0000000IHDR")
	announcementJPEGBytes = []byte("\xff\xd8\xff\xe00000JFIF")
	announcementGIFBytes  = []byte("GIF89a00000")
	announcementWEBPBytes = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 16)...)
)

func TestDecodeAnnouncementImageDataURI(t *testing.T) {
	tests := []struct {
		name       string
		dataURI    string
		wantMime   string
		wantErrMsg string
	}{
		{name: "png", dataURI: announcementImageDataURI("image/png", announcementPNGBytes), wantMime: "image/png"},
		{name: "jpeg", dataURI: announcementImageDataURI("image/jpeg", announcementJPEGBytes), wantMime: "image/jpeg"},
		{name: "gif", dataURI: announcementImageDataURI("image/gif", announcementGIFBytes), wantMime: "image/gif"},
		{name: "webp", dataURI: announcementImageDataURI("image/webp", announcementWEBPBytes), wantMime: "image/webp"},
		{
			// 声明与魔数不一致但都在白名单内：以魔数为准
			name:     "declared png actual jpeg",
			dataURI:  announcementImageDataURI("image/png", announcementJPEGBytes),
			wantMime: "image/jpeg",
		},
		{name: "svg rejected", dataURI: announcementImageDataURI("image/svg+xml", []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")), wantErrMsg: "图片必须为 PNG、JPG、GIF 或 WebP 格式"},
		{name: "text content rejected", dataURI: announcementImageDataURI("image/png", []byte("plain text not an image")), wantErrMsg: "图片内容不是有效的 PNG、JPG、GIF 或 WebP"},
		{name: "not a data uri", dataURI: "https://example.com/a.png", wantErrMsg: "图片必须为 data:image/...;base64 格式"},
		{name: "missing base64 marker", dataURI: "data:image/png," + base64.StdEncoding.EncodeToString(announcementPNGBytes), wantErrMsg: "图片必须为 data:image/...;base64 格式"},
		{name: "bad base64", dataURI: "data:image/png;base64,!!not-base64!!", wantErrMsg: "图片 base64 解码失败"},
		{
			name:       "oversize",
			dataURI:    announcementImageDataURI("image/png", bytes.Repeat([]byte{0}, maxAnnouncementImageBytes+1)),
			wantErrMsg: "图片不能超过5MB",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mime, _, errMsg := decodeAnnouncementImageDataURI(tc.dataURI)
			if tc.wantErrMsg != "" {
				assert.Equal(t, tc.wantErrMsg, errMsg)
				assert.Empty(t, mime)
				return
			}
			assert.Empty(t, errMsg)
			assert.Equal(t, tc.wantMime, mime)
		})
	}
}

func TestAnnouncementImageNameRe(t *testing.T) {
	validName := strings.Repeat("ab", 16) + ".png"
	for _, name := range []string{validName, strings.Repeat("0f", 16) + ".jpg", strings.Repeat("0f", 16) + ".gif", strings.Repeat("0f", 16) + ".webp"} {
		assert.True(t, announcementImageNameRe.MatchString(name), name)
	}
	for _, name := range []string{
		"../" + validName,                      // 路径穿越
		strings.Repeat("0f", 16) + ".png/x",    // 带路径
		strings.Repeat("0F", 16) + ".png",      // 大写不在生成形态内
		strings.Repeat("0f", 15) + ".png",      // 长度不符
		"abc.png",                              // 非生成形态
		strings.Repeat("0f", 16) + ".svg",      // 扩展名白名单外
		strings.Repeat("0f", 16),               // 无扩展名
		strings.Repeat("0f", 16) + ".php5.png", // 双扩展名
	} {
		assert.False(t, announcementImageNameRe.MatchString(name), name)
	}
}

func TestAnnouncementCursorRoundTrip(t *testing.T) {
	cursor := encodeAnnouncementCursor(1759900000, 42)
	pt, id, err := decodeAnnouncementCursor(cursor)
	require.NoError(t, err)
	assert.Equal(t, int64(1759900000), pt)
	assert.Equal(t, 42, id)

	for _, bad := range []string{"!!!", encodeAnnouncementCursor(0, 42), encodeAnnouncementCursor(100, 0), "MTAw"} {
		_, _, err := decodeAnnouncementCursor(bad)
		assert.Error(t, err, bad)
	}
}

type announcementListAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Items      []model.AnnouncementSummary `json:"items"`
		NextCursor string                      `json:"next_cursor"`
	} `json:"data"`
}

func callAnnouncementList(t *testing.T, query string) announcementListAPIResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/announcement/list?"+query, nil)
	GetAnnouncementList(c)
	var resp announcementListAPIResponse
	require.NoError(t, common.UnmarshalJsonStr(w.Body.String(), &resp))
	return resp
}

func TestGetAnnouncementListCursorPagination(t *testing.T) {
	setupAnnouncementControllerTest(t)
	gin.SetMode(gin.TestMode)

	// (id 递增即创建顺序) 期望可见顺序按 (publish_time, id) 倒序：2,4,3,1
	fixtures := []model.Announcement{
		{App: "qianmiao", Title: "t1", PublishTime: 100, Enabled: true},
		{App: "qianmiao", Title: "t2", PublishTime: 103, Enabled: true},
		{App: "qianmiao", Title: "t3", PublishTime: 102, Enabled: true},
		{App: "qianmiao", Title: "t4", PublishTime: 103, Enabled: true},                           // 与 t2 并列，按 id 靠后
		{App: "qianmiao", Title: "t5", PublishTime: 101, Enabled: false},                          // 停用不可见
		{App: "qianmiao", Title: "t6", PublishTime: common.GetTimestamp() + 10000, Enabled: true}, // 未发布不可见
		{App: "other", Title: "t7", PublishTime: 104, Enabled: true},                              // 其他应用隔离
	}
	for i := range fixtures {
		require.NoError(t, model.DB.Create(&fixtures[i]).Error)
	}
	byTitle := map[string]int{}
	for _, f := range fixtures {
		byTitle[f.Title] = f.Id
	}

	// 缺 app 参数
	resp := callAnnouncementList(t, "")
	assert.False(t, resp.Success)

	// 非法 cursor（"!!!" 不经 URL 编码变形，且非合法 base64url）
	resp = callAnnouncementList(t, "app=qianmiao&cursor=!!!bad-cursor!!!")
	assert.False(t, resp.Success)
	assert.Equal(t, "cursor 参数非法", resp.Message)

	// 首页（t2/t4 的 publish_time 并列，id 大者在前）
	resp = callAnnouncementList(t, "app=qianmiao&limit=2")
	assert.True(t, resp.Success)
	require.Len(t, resp.Data.Items, 2)
	assert.Equal(t, byTitle["t4"], resp.Data.Items[0].Id)
	assert.Equal(t, byTitle["t2"], resp.Data.Items[1].Id)
	assert.NotEmpty(t, resp.Data.NextCursor)

	// 第二页（含并列 publish_time 的 keyset 边界）
	resp = callAnnouncementList(t, "app=qianmiao&limit=2&cursor="+resp.Data.NextCursor)
	assert.True(t, resp.Success)
	require.Len(t, resp.Data.Items, 2)
	assert.Equal(t, byTitle["t3"], resp.Data.Items[0].Id)
	assert.Equal(t, byTitle["t1"], resp.Data.Items[1].Id)

	// 恰好取满上一页 → 仍会给 cursor，但下一页为空并返回空 cursor
	resp = callAnnouncementList(t, "app=qianmiao&limit=2&cursor="+resp.Data.NextCursor)
	assert.True(t, resp.Success)
	assert.Empty(t, resp.Data.Items)
	assert.Empty(t, resp.Data.NextCursor)

	// 不带 limit（默认 20）一次取全，且总数为 4（过滤停用/未发布/其他应用）
	resp = callAnnouncementList(t, "app=qianmiao")
	assert.True(t, resp.Success)
	assert.Len(t, resp.Data.Items, 4)
	assert.Empty(t, resp.Data.NextCursor)
}

func TestAdminListOmitsContentButDetailIncludesIt(t *testing.T) {
	setupAnnouncementControllerTest(t)
	gin.SetMode(gin.TestMode)
	content := "# 正文标题\n\n![](/api/announcement/images/a.png)"
	created := model.Announcement{App: "qianmiao", Title: "t", Content: content, Type: "default", PublishTime: 100, Enabled: true}
	require.NoError(t, model.DB.Create(&created).Error)

	// 管理列表：不含 content 正文，带管理面需要的状态字段
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/announcement/admin/list", nil)
	AdminListAnnouncements(c)
	body := w.Body.String()
	assert.Contains(t, body, `"title":"t"`)
	assert.Contains(t, body, `"enabled":true`)
	assert.NotContains(t, body, "正文标题")
	assert.NotContains(t, body, `"content"`)

	// 管理详情：含 content（编辑表单用）
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Params = append(c2.Params, gin.Param{Key: "id", Value: strconv.Itoa(created.Id)})
	c2.Request = httptest.NewRequest("GET", "/api/announcement/admin/"+strconv.Itoa(created.Id), nil)
	AdminGetAnnouncementDetail(c2)
	assert.Contains(t, w2.Body.String(), "正文标题")
}
