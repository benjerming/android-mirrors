import { Outlet } from 'react-router-dom';

import { AppTopbar } from '@/components/layout/AppTopbar';

export function AppShell() {
  return (
    <div className="h-screen overflow-hidden bg-[radial-gradient(circle_at_top_left,_rgba(251,191,36,0.14),_transparent_32%),linear-gradient(180deg,_#f5f5f4_0%,_#e7e5e4_100%)] px-4 py-4 text-stone-900 md:px-6 md:py-6">
      <div className="flex h-full w-full flex-col gap-4">
        <AppTopbar />
        <main className="flex-1 min-h-0 overflow-y-auto rounded-[2rem] border border-white/70 bg-white/80 p-6 shadow-sm backdrop-blur">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
