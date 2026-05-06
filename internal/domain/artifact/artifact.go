// Package artifact 表示 APK / 资源文件的领域对象。
package artifact

import "time"

// Type 表示 artifact 类型。
type Type string

const (
	TypeAPK Type = "apk"
)

// Artifact 表示一份上传到服务器的二进制资源（当前只用于 APK）。
type Artifact struct {
	ID          uint
	UserID      uint
	Type        Type
	OriginName  string
	Path        string
	Size        int64
	SHA256      string
	PackageName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
