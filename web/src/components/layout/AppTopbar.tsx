import { LogOut } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { logout } from '@/features/session/hooks';

// AppTopbar 表示页面顶部的信息条，后面可以继续放登录状态、筛选条件和快捷操作。
export function AppTopbar() {
  return (
    <header className="flex flex-col gap-4 rounded-[2rem] border border-stone-200 bg-white/90 px-6 py-5 shadow-sm backdrop-blur md:flex-row md:items-center md:justify-between">
      <div>
        <p className="text-xs uppercase tracking-[0.3em] text-stone-500">Workspace</p>
        <h2 className="mt-2 text-2xl font-semibold text-stone-900">实例编排面板</h2>
      </div>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="rounded-full bg-emerald-50 px-4 py-2 text-sm font-medium text-emerald-700">会话鉴权已连接</div>
        <Button onClick={logout} size="sm" variant="outline">
          <LogOut className="h-4 w-4" />
          退出登录
        </Button>
      </div>
    </header>
  );
}
