import { RouterProvider } from 'react-router-dom';

import { AppProviders } from '@/components/providers/AppProviders';
import { useSessionLifecycle } from '@/features/session/hooks';
import { router } from '@/router';

// AppRouter 表示真正承载路由的那一层，顺手接上会话生命周期监听。
function AppRouter() {
  useSessionLifecycle();

  return <RouterProvider router={router} />;
}

// App 表示前端应用的总入口，负责把全局 providers 和路由接到一起。
export default function App() {
  return (
    <AppProviders>
      <AppRouter />
    </AppProviders>
  );
}
