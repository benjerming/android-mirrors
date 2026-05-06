// Package profile 提供 AVD 模板的值对象，对应 config.json 的 avdProfiles。
package profile

// Profile 表示 config.json 中的一条 AVD 模板（值对象，不入库）。
type Profile struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Device      string `json:"device"`
	Resolution  string `json:"resolution"`
	Density     int    `json:"density"`
}
