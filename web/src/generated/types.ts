// 本文件由 tygo 生成（见仓库根 tygo.yaml），请勿手改。
// 当前阶段尚未接入完整管道，先以手写镜像兜底；后端 DTO 落地后切回 `make gen-types`。

// Profile 对应 internal/domain/profile/profile.go 的 Profile。
export interface Profile {
  id: string;
  displayName: string;
  device: string;
  resolution: string;
  density: number;
}

// Language 对应 internal/domain/language/language.go 的 Language。
export interface Language {
  code: string;
  label: string;
}

// ConfigOptions 对应后端 GET /api/v1/configs/options 的响应体。
export interface ConfigOptions {
  profiles: Profile[];
  languages: Language[];
}

// AggregateState 对应 internal/domain/group.GroupStats.AggregateState() 的输出枚举。
export type AggregateState = 'all_running' | 'all_stopped' | 'partial' | 'transitioning' | 'error';

// GroupSummary 对应 GET /api/v1/groups 的列表行。
export interface GroupSummary {
  id: number;
  name: string;
  profileId: string;
  profileDisplayName: string;
  instanceCount: number;
  runningCount: number;
  errorCount: number;
  aggregateState: AggregateState;
}

// GroupCreateInstance 对应创建分组响应里 instances[] 的最小字段。
export interface GroupCreateInstance {
  id: number;
  name: string;
  status: string;
  language?: string;
}

// GroupCreateFailure 对应创建分组响应里 failed[] 的失败项。
export interface GroupCreateFailure {
  language: string;
  error: string;
}

// GroupCreateResult 对应 POST /api/v1/groups 的响应。
export interface GroupCreateResult {
  group: GroupSummary;
  instances: GroupCreateInstance[];
  failed: GroupCreateFailure[];
}

// GroupActionResult 对应 POST /api/v1/groups/:id/start|stop 的响应（202 Accepted）。
// transitioning：本次刚被派发到后台的实例 id（status 已置 starting/stopping）。
// skipped：当前已 running/starting 或 stopped/stopping，幂等跳过。
// failed：派发阶段失败列表；运行期 runner 错误不在此处暴露，会以 status='error' 反映在 GET /groups。
export interface GroupActionResult {
  transitioning: number[];
  skipped: number[];
  failed: { instanceId: number; error: string }[];
}

// GroupInstance 对应单条实例（镜像页主屏 / 副屏使用）。
export interface GroupInstance {
  id: number;
  name: string;
  tag: string;
  mode: string;
  status: string;
  templateId: number;
  language?: string;
}

// GroupDetail 对应 GET /api/v1/groups/:id 的响应体。
export interface GroupDetail {
  group: GroupSummary;
  instances: GroupInstance[];
}
