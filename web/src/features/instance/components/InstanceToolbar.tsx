import { ChevronDown, MonitorPlay, Plus, Search } from 'lucide-react';

import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import type { InstanceDTO } from '@/features/instance/api';

type InstanceToolbarProps = {
  keyword: string;
  selectedCount: number;
  canEnterMirror: boolean;
  selectedInstances: InstanceDTO[];
  onKeywordChange: (value: string) => void;
  onCreateClick: () => void;
  onEnterMirror: () => void;
};

// buildMirrorHint 用来把“为什么现在不能进入镜像”翻译成人能看懂的一句话。
function buildMirrorHint(selectedInstances: InstanceDTO[]) {
  if (selectedInstances.length === 0) {
    return '先选择至少 1 台实例，才能进入镜像页。';
  }

  if (selectedInstances.some((instance) => instance.status !== 'running')) {
    return '已选实例里有未运行的机器，当前不能一起进入镜像。';
  }

  if (new Set(selectedInstances.map((instance) => instance.templateId)).size > 1) {
    return '多选进入镜像时，所有实例必须来自同一个模板。';
  }

  return '已满足进入镜像条件。';
}

// InstanceToolbar 表示实例页顶部工具栏，负责搜索、创建和进入镜像入口。
export function InstanceToolbar({
  keyword,
  selectedCount,
  canEnterMirror,
  selectedInstances,
  onKeywordChange,
  onCreateClick,
  onEnterMirror,
}: InstanceToolbarProps) {
  const mirrorHint = buildMirrorHint(selectedInstances);

  return (
    <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
      <div className="space-y-3">
        <div>
          <p className="text-sm uppercase tracking-[0.3em] text-stone-500">Instances</p>
          <h1 className="mt-2 text-3xl font-semibold text-stone-950">实例列表</h1>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-stone-600">
            在这里集中筛选实例、批量选择同模板运行机，并把它们带进下一阶段的镜像操作页。
          </p>
        </div>
        <label className="relative block w-full max-w-md">
          <Search className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-stone-400" />
          <Input
            className="pl-11"
            onChange={(event) => onKeywordChange(event.target.value)}
            placeholder="按名称或标签搜索实例"
            value={keyword}
          />
        </label>
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-end">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button disabled={selectedCount === 0} type="button" variant="outline">
              已选 {selectedCount} 台
              <ChevronDown className="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>当前选中</DropdownMenuLabel>
            {selectedInstances.length === 0 ? (
              <DropdownMenuItem>还没有选择实例</DropdownMenuItem>
            ) : (
              selectedInstances.map((instance) => (
                <DropdownMenuItem key={instance.id}>{`${instance.name} · ${instance.status === 'running' ? '运行中' : '已停止'}`}</DropdownMenuItem>
              ))
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem>{mirrorHint}</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        <Button onClick={onCreateClick} type="button" variant="outline">
          <Plus className="h-4 w-4" />
          新建实例
        </Button>

        <TooltipProvider delayDuration={120}>
          <Tooltip>
            <TooltipTrigger asChild>
              <span tabIndex={0}>
                <Button disabled={!canEnterMirror} onClick={onEnterMirror} type="button">
                  <MonitorPlay className="h-4 w-4" />
                  {`进入镜像 (${selectedCount})`}
                </Button>
              </span>
            </TooltipTrigger>
            {!canEnterMirror ? <TooltipContent>{mirrorHint}</TooltipContent> : null}
          </Tooltip>
        </TooltipProvider>
      </div>
    </div>
  );
}
