// Package language 提供 APK locale 的值对象，对应 config.json 的 languages。
package language

// Language 表示一种 APK locale，对应 config.json 的 languages 数组。
type Language struct {
	Code  string `json:"code"`  // BCP-47，例如 zh-CN
	Label string `json:"label"` // 中文显示名，例如"简体中文"
}
