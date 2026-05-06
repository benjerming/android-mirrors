import { Navigate, createBrowserRouter, redirect } from 'react-router-dom';

import { AppShell } from '@/components/layout/AppShell';
import { clearSessionToken, fetchSessionProfile, readSessionToken } from '@/features/session/api';
import { GroupsPage } from '@/pages/GroupsPage';
import { InstancesPage } from '@/pages/InstancesPage';
import { LoginPage } from '@/pages/LoginPage';
import { MirrorPage } from '@/pages/MirrorPage';
import { NotFoundPage } from '@/pages/NotFoundPage';

// requireAuth 用来拦截需要登录的页面，没有 token 或 token 已失效时统一回到登录页。
export async function requireAuth() {
  const token = readSessionToken();

  if (!token) {
    throw redirect('/login');
  }

  try {
    await fetchSessionProfile();
    return null;
  } catch {
    clearSessionToken();
    throw redirect('/login');
  }
}

// router 保存页面与路径的对应关系，阶段 1 先把基础壳与核心页面入口搭起来。
export const router = createBrowserRouter([
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    path: '/',
    loader: requireAuth,
    element: <AppShell />,
    children: [
      {
        index: true,
        element: <Navigate replace to="/groups" />,
      },
      {
        path: 'groups',
        element: <GroupsPage />,
      },
      {
        path: 'groups/:groupId/mirror',
        element: <MirrorPage />,
      },
      {
        path: 'instances',
        element: <InstancesPage />,
      },
      {
        path: 'instances/mirror',
        element: <MirrorPage />,
      },
    ],
  },
  {
    path: '*',
    element: <NotFoundPage />,
  },
]);
