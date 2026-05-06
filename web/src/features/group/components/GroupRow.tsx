import { LoaderCircle, Play, Square } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router-dom';

import { Button } from '@/components/ui/button';
import { useStartGroup, useStopGroup } from '@/features/group/api';
import { DeleteGroupConfirm } from '@/features/group/components/DeleteGroupConfirm';
import { RenameGroupDialog } from '@/features/group/components/RenameGroupDialog';
import type { AggregateState, GroupSummary } from '@/generated/types';

interface GroupRowProps {
  group: GroupSummary;
}

// statusLabel 用英文枚举做比较（spec §14：禁止用中文字面量）；只在最终展示阶段映射成中文文案。
function statusLabel(state: AggregateState, runningCount: number, instanceCount: number): string {
  switch (state) {
    case 'all_running':
      return '全部运行中';
    case 'all_stopped':
      return instanceCount === 0 ? '空分组' : '全部已停止';
    case 'partial':
      return `${runningCount}/${instanceCount} 运行中`;
    case 'transitioning':
      return `启动中 ${runningCount}/${instanceCount} 已就绪`;
    case 'error':
      return '部分异常';
  }
}

// primaryActionLabel 决定行尾主按钮文案：根据聚合态分别显示"全部启动"、"全部停止"、"启动剩余"。
function primaryActionLabel(state: AggregateState): string {
  if (state === 'all_running') return '全部停止';
  if (state === 'partial') return '启动剩余';
  return '全部启动';
}

// GroupRow 表示分组列表中的一行，集成主操作（启停）+ 进入镜像 + 重命名 + 删除。
export function GroupRow({ group }: GroupRowProps) {
  const startMutation = useStartGroup();
  const stopMutation = useStopGroup();
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const transitioning = group.aggregateState === 'transitioning';
  const busy = startMutation.isPending || stopMutation.isPending || transitioning;
  const canEnterMirror = group.aggregateState !== 'all_stopped' && group.instanceCount > 0;

  function handlePrimary() {
    if (group.aggregateState === 'all_running') {
      stopMutation.mutate(group.id);
    } else {
      startMutation.mutate(group.id);
    }
  }

  return (
    <div
      data-testid={`group-row-${group.id}`}
      className="flex flex-col gap-3 rounded-2xl border border-stone-200/70 bg-white/85 p-4 shadow-sm md:flex-row md:items-center md:justify-between"
    >
      <div className="space-y-1">
        <div className="flex items-baseline gap-2">
          <h3 className="text-base font-semibold text-stone-900">{group.name}</h3>
          <span className="text-xs text-stone-500">
            {group.profileDisplayName || group.profileId}
          </span>
        </div>
        <p data-testid="aggregate-state" className="text-sm text-stone-600">
          {statusLabel(group.aggregateState, group.runningCount, group.instanceCount)}
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Button data-testid="primary-action" onClick={handlePrimary} disabled={busy} size="sm">
          {busy ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : group.aggregateState === 'all_running' ? (
            <Square className="h-4 w-4" />
          ) : (
            <Play className="h-4 w-4" />
          )}
          {primaryActionLabel(group.aggregateState)}
        </Button>
        <Button asChild size="sm" variant="secondary">
          <Link
            data-testid="enter-mirror"
            to={`/groups/${group.id}/mirror`}
            aria-disabled={!canEnterMirror}
            className={!canEnterMirror ? 'pointer-events-none opacity-50' : undefined}
            onClick={(e) => {
              if (!canEnterMirror) e.preventDefault();
            }}
          >
            进入镜像
          </Link>
        </Button>
        <Button
          data-testid="rename-action"
          onClick={() => setRenameOpen(true)}
          size="sm"
          variant="outline"
        >
          重命名
        </Button>
        <Button
          data-testid="delete-action"
          onClick={() => setDeleteOpen(true)}
          size="sm"
          variant="outline"
        >
          删除
        </Button>
      </div>
      <RenameGroupDialog
        groupId={group.id}
        currentName={group.name}
        open={renameOpen}
        onOpenChange={setRenameOpen}
      />
      <DeleteGroupConfirm
        groupId={group.id}
        groupName={group.name}
        instanceCount={group.instanceCount}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
      />
    </div>
  );
}
