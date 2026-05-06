import { Link } from 'react-router-dom';

// NotFoundPage 表示未命中路由时的兜底页，避免用户走到空白地址后不知道怎么返回。
export function NotFoundPage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-stone-100 px-6 py-10">
      <section className="w-full max-w-lg rounded-[2rem] border border-stone-200 bg-white p-8 text-center shadow-sm">
        <p className="text-sm uppercase tracking-[0.3em] text-stone-500">404</p>
        <h1 className="mt-3 text-3xl font-semibold text-stone-900">页面不存在</h1>
        <p className="mt-3 text-sm leading-6 text-stone-600">当前地址还没有对应页面，你可以先回到实例列表继续操作。</p>
        <Link
          className="mt-6 inline-flex items-center rounded-full bg-stone-900 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-stone-700"
          to="/instances"
        >
          返回实例列表
        </Link>
      </section>
    </main>
  );
}
