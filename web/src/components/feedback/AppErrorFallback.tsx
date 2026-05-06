import type { FallbackProps } from 'react-error-boundary';

// AppErrorFallback 表示全局兜底报错页，避免页面崩掉后只剩空白。
export function AppErrorFallback({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <main className="flex min-h-screen items-center justify-center bg-stone-100 px-6 py-12">
      <section className="w-full max-w-xl rounded-3xl border border-stone-200 bg-white p-8 shadow-sm">
        <p className="text-sm font-medium uppercase tracking-[0.3em] text-amber-700">Application Error</p>
        <h1 className="mt-4 text-3xl font-semibold text-stone-900">页面暂时打不开</h1>
        <p className="mt-3 text-sm leading-6 text-stone-600">
          我们已经拦住了这次异常，避免整个应用一起失效。你可以先重试，如果还不行再检查接口或控制台日志。
        </p>
        <pre className="mt-6 overflow-x-auto rounded-2xl bg-stone-950/95 p-4 text-xs leading-6 text-stone-100">
          {error.message}
        </pre>
        <button
          className="mt-6 inline-flex items-center rounded-full bg-stone-900 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-stone-700"
          onClick={resetErrorBoundary}
          type="button"
        >
          重新加载当前页面
        </button>
      </section>
    </main>
  );
}
