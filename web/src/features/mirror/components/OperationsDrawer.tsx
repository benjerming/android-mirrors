import { X } from 'lucide-react';
import type { ReactNode } from 'react';

import { Button } from '@/components/ui/button';

interface OperationsDrawerProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}

// OperationsDrawer 表示镜像页右侧抽屉容器（按键 / APK / 文件三个区由 children 注入）。
export function OperationsDrawer({ open, onClose, children }: OperationsDrawerProps) {
  if (!open) return null;
  return (
    <div data-testid="operations-drawer" className="fixed inset-0 z-40 flex justify-end">
      <button
        type="button"
        aria-label="关闭抽屉"
        className="flex-1 bg-stone-950/40"
        onClick={onClose}
      />
      <aside className="flex w-full max-w-md flex-col gap-4 overflow-y-auto bg-white p-5 shadow-xl">
        <header className="flex items-center justify-between">
          <h3 className="text-base font-semibold text-stone-900">操作面板</h3>
          <Button variant="outline" size="sm" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </header>
        {children}
      </aside>
    </div>
  );
}
