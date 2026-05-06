import type { ReactNode } from 'react';

import { QueryClientProvider } from '@tanstack/react-query';
import { ErrorBoundary } from 'react-error-boundary';
import { Toaster } from 'sonner';

import { AppErrorFallback } from '@/components/feedback/AppErrorFallback';
import { queryClient } from '@/lib/queryClient';

type AppProvidersProps = {
  children: ReactNode;
};

// AppProviders 用来集中挂载全局能力，避免每个页面重复接查询、报错和提示组件。
export function AppProviders({ children }: AppProvidersProps) {
  return (
    <ErrorBoundary FallbackComponent={AppErrorFallback}>
      <QueryClientProvider client={queryClient}>
        {children}
        <Toaster richColors position="bottom-right" />
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
