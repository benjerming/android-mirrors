// AuthTopbar 表示登录页顶部的品牌说明区，用来告诉用户当前页面要做什么。
export function AuthTopbar() {
  return (
    <header className="space-y-4">
      <p className="text-xs uppercase tracking-[0.35em] text-amber-700/80">Assassin Controller</p>
      <div className="space-y-3">
        <h1 className="text-4xl font-semibold tracking-tight text-stone-950">把控制台会话先接通，再进入实例编排。</h1>
        <p className="max-w-xl text-base leading-7 text-stone-600">
          阶段 2 先把登录、令牌保存、401 清理和多标签同步打稳，后面列表页与镜像页就可以直接复用这条会话链路。
        </p>
      </div>
    </header>
  );
}
