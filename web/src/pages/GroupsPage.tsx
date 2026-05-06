import { Skeleton } from '@/components/ui/skeleton';
import { CreateGroupDialog } from '@/features/group/components/CreateGroupDialog';
import { GroupList } from '@/features/group/components/GroupList';
import { useGroups } from '@/features/group/api';

// GroupsPage 表示 /groups 列表入口，承载空态、加载、列表三种状态。
export function GroupsPage() {
  const { data, isLoading, isError } = useGroups();

  return (
    <section className="space-y-6">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-stone-900">分组</h2>
          <p className="text-sm text-stone-600">每个分组对应一组同模板、不同语言的模拟器。</p>
        </div>
        <CreateGroupDialog />
      </header>

      {isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      ) : isError ? (
        <p className="text-sm text-rose-600">加载分组失败，请稍后重试。</p>
      ) : data && data.length === 0 ? (
        <div
          data-testid="groups-empty"
          className="rounded-2xl border border-dashed border-stone-300 bg-white/70 p-10 text-center text-stone-600"
        >
          <p className="text-base font-medium">还没有分组</p>
          <p className="mt-1 text-sm">点击右上角「新建分组」开始。</p>
        </div>
      ) : (
        <GroupList groups={data ?? []} />
      )}
    </section>
  );
}
