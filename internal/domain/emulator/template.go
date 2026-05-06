package emulator

import "time"

// Template 表示一个可用于建机的模拟器模板，前端新建实例弹窗会读取它。
type Template struct {
	ID          uint      // ID 表示模板的唯一编号，前端创建实例时会传回这个值。
	Name        string    // Name 表示模板名称，列表和下拉框会直接显示它。
	Description string    // Description 表示给人看的模板说明，帮助区分不同机型或系统版本。
	SystemImage string    // SystemImage 表示底层使用的 Android system image 标识。
	Device      string    // Device 表示 avdmanager 创建 AVD 时使用的设备型号（如 small_phone）。
	Resolution  string    // Resolution 表示模拟器屏幕分辨率，如 "1080x2400"。
	Density     int       // Density 表示模拟器屏幕像素密度（DPI）。
	CreatedAt   time.Time // CreatedAt 表示模板记录的创建时间。
	UpdatedAt   time.Time // UpdatedAt 表示模板记录的最近更新时间。
}
