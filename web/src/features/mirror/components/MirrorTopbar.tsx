import { ChevronLeft } from 'lucide-react';
import { Link } from 'react-router-dom';

import { Button } from '@/components/ui/button';
import { useControlStore } from '@/stores/control';

interface MirrorTopbarProps {
  groupName: string;
  runningCount: number;
  instanceCount: number;
  onOpenDrawer: () => void;
}

export function MirrorTopbar({ groupName, runningCount, instanceCount, onOpenDrawer }: MirrorTopbarProps) {
  const mode = useControlStore((s) => s.mode);
  const setMode = useControlStore((s) => s.setMode);

  const isSingle = mode === 'single';
  const containerClass = isSingle
    ? 'rounded-2xl bg-orange-900 px-4 py-3 text-orange-50'
    : 'rounded-2xl bg-stone-100 px-4 py-3 text-stone-900';

  return (
    <header data-testid="mirror-topbar" className={containerClass}>
      <div className="flex items-center justify-between gap-3">
        <Button asChild variant="outline" size="sm">
          <Link to="/groups">
            <ChevronLeft className="h-4 w-4" />
            返回
          </Link>
        </Button>
        <div className="flex-1 text-center">
          <p className="text-sm font-semibold">{groupName}</p>
          <p data-testid="mirror-progress" className="text-xs opacity-80">
            {`运行中 ${runningCount}/${instanceCount}${isSingle ? ' · 独控模式' : ''}`}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div
            role="group"
            aria-label="控制模式"
            className="inline-flex overflow-hidden rounded-md border border-current/30"
          >
            <button
              type="button"
              data-testid="mode-broadcast"
              aria-pressed={!isSingle}
              onClick={() => setMode('broadcast')}
              className={`px-3 py-1 text-xs ${
                !isSingle ? 'bg-stone-900 text-stone-50' : 'bg-transparent text-current'
              }`}
            >
              群控
            </button>
            <button
              type="button"
              data-testid="mode-single"
              aria-pressed={isSingle}
              onClick={() => setMode('single')}
              className={`px-3 py-1 text-xs ${
                isSingle ? 'bg-orange-50 text-orange-900' : 'bg-transparent text-current'
              }`}
            >
              独控
            </button>
          </div>
          <Button
            size="sm"
            variant="secondary"
            onClick={onOpenDrawer}
            disabled={isSingle}
          >
            操作面板
          </Button>
        </div>
      </div>
    </header>
  );
}
