import { CheckCircle2, Circle, MonitorSmartphone, SquareTerminal, Trash2 } from 'lucide-react';
import type { MouseEvent } from 'react';

import { Card } from '@/components/ui/card';
import type { InstanceDTO } from '@/features/instance/api';
import { cn } from '@/lib/utils';

type InstanceCardProps = {
  instance: InstanceDTO;
  selected: boolean;
  onToggle: (id: number) => void;
  onDelete: (instance: InstanceDTO) => void;
  isDeleting?: boolean;
};

const MODE_LABELS: Record<InstanceDTO['mode'], string> = {
  reusable: '可复用',
  ephemeral: '临时机',
  debug: '调试机',
};

const STATUS_LABELS: Record<InstanceDTO['status'], string> = {
  running: '运行中',
  stopped: '已停止',
};

// InstanceCard 表示实例列表里的单张卡片，负责展示状态并响应选中操作。
export function InstanceCard({ instance, selected, onToggle, onDelete, isDeleting }: InstanceCardProps) {
  const running = instance.status === 'running';

  // handleDeleteClick 用来在删除按钮上拦截事件冒泡，避免顺手触发整张卡片的选中切换。
  function handleDeleteClick(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation();
    onDelete(instance);
  }

  return (
    <Card
      aria-pressed={selected}
      className={cn(
        'group relative h-full border-stone-200/80 bg-white/95 shadow-sm transition hover:-translate-y-0.5 hover:shadow-lg',
        selected ? 'border-amber-400 ring-2 ring-amber-200' : 'hover:border-stone-300',
      )}
    >
      <button
        aria-label={`删除实例 ${instance.name}`}
        className="absolute right-4 top-4 z-10 inline-flex h-8 w-8 items-center justify-center rounded-full bg-white/80 text-stone-400 shadow-sm ring-1 ring-stone-200 transition hover:bg-rose-50 hover:text-rose-600 disabled:cursor-not-allowed disabled:opacity-60"
        disabled={isDeleting}
        onClick={handleDeleteClick}
        type="button"
      >
        <Trash2 className="h-4 w-4" />
      </button>
      <button
        className="flex h-full w-full flex-col gap-5 rounded-[2rem] p-5 text-left"
        onClick={() => onToggle(instance.id)}
        type="button"
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 space-y-2">
            <div className="flex items-center gap-2">
              <span
                className={cn(
                  'inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold',
                  running ? 'bg-emerald-100 text-emerald-800' : 'bg-stone-200 text-stone-700',
                )}
              >
                {STATUS_LABELS[instance.status]}
              </span>
              <span className="inline-flex items-center rounded-full bg-amber-50 px-3 py-1 text-xs font-medium text-amber-900">
                {MODE_LABELS[instance.mode]}
              </span>
            </div>
            <div>
              <h3 className="truncate text-xl font-semibold text-stone-950">{instance.name}</h3>
              <p className="mt-1 text-sm text-stone-500">{`标签：${instance.tag}`}</p>
            </div>
          </div>
          <div className="shrink-0 text-amber-500">
            {selected ? <CheckCircle2 className="h-6 w-6" /> : <Circle className="h-6 w-6" />}
          </div>
        </div>

        <dl className="grid gap-3 rounded-[1.5rem] bg-stone-50 p-4 text-sm text-stone-600">
          <div className="flex items-center justify-between gap-3">
            <dt className="inline-flex items-center gap-2">
              <MonitorSmartphone className="h-4 w-4" />
              模板编号
            </dt>
            <dd className="font-medium text-stone-900">{instance.templateId}</dd>
          </div>
          <div className="flex items-center justify-between gap-3">
            <dt className="inline-flex items-center gap-2">
              <SquareTerminal className="h-4 w-4" />
              镜像资格
            </dt>
            <dd className={cn('font-medium', running ? 'text-emerald-700' : 'text-stone-500')}>{running ? '可进入' : '需先启动'}</dd>
          </div>
        </dl>

        <span className="sr-only">{`选择实例 ${instance.name}`}</span>
      </button>
    </Card>
  );
}
