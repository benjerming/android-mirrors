package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"assassin-android-controller/internal/application/config"
	"assassin-android-controller/internal/interfaces/httpapi"

	"github.com/gin-gonic/gin"
)

func TestConfigHandler_Options(t *testing.T) {
	gin.SetMode(gin.TestMode)
	abs, err := filepath.Abs("../../../configs/config.json")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	svc, err := config.New(abs)
	if err != nil {
		t.Fatal(err)
	}

	h := httpapi.NewConfigHandler(svc)
	r := gin.New()
	r.GET("/configs/options", h.Options)

	req := httptest.NewRequest(http.MethodGet, "/configs/options", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Profiles  []map[string]any `json:"profiles"`
		Languages []map[string]any `json:"languages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Profiles) == 0 || len(body.Languages) == 0 {
		t.Fatalf("empty options: %+v", body)
	}
	// 抽样验证字段保留：Profile 应有 id/displayName，Language 应有 code/label。
	if _, ok := body.Profiles[0]["id"]; !ok {
		t.Errorf("profile missing id: %+v", body.Profiles[0])
	}
	if _, ok := body.Languages[0]["code"]; !ok {
		t.Errorf("language missing code: %+v", body.Languages[0])
	}
}
