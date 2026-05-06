// HomePage 表示登录后的主页面占位组件，后续真正功能会从这里展开。
export function HomePage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-zinc-100 px-6">
      <section className="w-full max-w-3xl rounded-3xl border border-zinc-200 bg-white p-10 shadow-sm">
        <h1 className="text-3xl font-semibold text-zinc-900">Home Page Placeholder</h1>
        <p className="mt-3 text-base text-zinc-600">This page will receive the dashboard content later.</p>
      </section>
    </main>
  );
}
