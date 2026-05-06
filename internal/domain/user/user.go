// Package user 定义"登录用户"这个领域概念。
//
// 它承载的是"谁拥有这些实例"这层归属关系，所有跨用户的权限校验都从这里出发。
// 与基础设施和 HTTP 解耦：本包不负责持久化和接口序列化，只描述用户在业务上的样子。
package user

import "time"

// User 表示一个登录用户，它负责承接“谁拥有这些实例”这层归属关系。
type User struct {
	ID        uint      // ID 表示用户在数据库里的唯一编号，后续实例归属都靠它判断。
	Username  string    // Username 表示用户登录时输入的名字，前端顶部栏会显示它。
	CreatedAt time.Time // CreatedAt 表示这条用户记录是什么时候创建的，方便排查数据来源。
	UpdatedAt time.Time // UpdatedAt 表示这条用户记录最近一次被更新的时间。
}
