// @deprecated F-17 / M21：此页面已弃用，仅保留只读浏览给运维。
//
// 行为变化：
// - 顶部显示横幅说明；
// - 移除「+ 新建实例」「批量进入镜像」「单实例删除」三个 UI 入口（spec §13）；
// - 单实例启停菜单项保留（运维定位问题用）；
// - 数据来源仍是 `GET /api/v1/instances`（B-33 阶段保留）。
import { LoaderCircle, Play, Square } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useInstancesQuery } from '@/features/instance/hooks';
import { useStartInstance, useStopInstance } from '@/features/instance/mutations';

export function InstancesPage() {
  const instancesQuery = useInstancesQuery();
  const startMutation = useStartInstance();
  const stopMutation = useStopInstance();
  const busy = startMutation.isPending || stopMutation.isPending;

  return (
    <section className="space-y-4">
      <div
        data-testid="instances-deprecated-banner"
        className="rounded-2xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900"
      >
        本页面已弃用，请使用「📱 分组」管理实例；此处仅保留只读浏览。
      </div>

      <header>
        <h2 className="text-2xl font-semibold text-stone-900">所有实例（只读）</h2>
        <p className="text-sm text-stone-600">仅供运维定位问题；新建 / 删除 / 批量进入镜像入口已移除。</p>
      </header>

      {instancesQuery.isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : instancesQuery.isError ? (
        <p className="text-sm text-rose-600">加载实例列表失败，请稍后重试。</p>
      ) : (instancesQuery.data ?? []).length === 0 ? (
        <p className="text-sm text-stone-500">尚未创建任何实例。</p>
      ) : (
        <ul className="space-y-2">
          {(instancesQuery.data ?? []).map((inst) => (
            <li
              key={inst.id}
              data-testid={`legacy-instance-${inst.id}`}
              className="flex items-center justify-between rounded-2xl border border-stone-200 bg-white/85 px-4 py-3"
            >
              <div className="space-y-1">
                <p className="text-sm font-medium text-stone-900">{inst.name}</p>
                <p className="text-xs text-stone-500">
                  {inst.mode} · {inst.status}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  data-testid={`legacy-toggle-${inst.id}`}
                  size="sm"
                  variant="outline"
                  disabled={busy}
                  onClick={() => {
                    if (inst.status === 'running') stopMutation.mutate(inst.id);
                    else startMutation.mutate(inst.id);
                  }}
                >
                  {busy ? (
                    <LoaderCircle className="h-3 w-3 animate-spin" />
                  ) : inst.status === 'running' ? (
                    <Square className="h-3 w-3" />
                  ) : (
                    <Play className="h-3 w-3" />
                  )}
                  {inst.status === 'running' ? '停止' : '启动'}
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
