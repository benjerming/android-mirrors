// queryKeys 统一保存查询缓存 key，避免后面多人开发时出现拼写不一致的问题。
export const queryKeys = {
  session: ['session'] as const,
  configs: {
    options: ['configs', 'options'] as const,
  },
  groups: {
    all: ['groups'] as const,
    detail: (id: number) => ['groups', id] as const,
  },
  instances: {
    all: ['instances'] as const,
    detail: (id: number) => ['instances', id] as const,
  },
  templates: ['templates'] as const,
  artifacts: {
    apkHistory: ['artifacts', 'apk-history'] as const,
  },
  apps: {
    installed: (instanceId: number) => ['apps', 'installed', instanceId] as const,
  },
} as const;
