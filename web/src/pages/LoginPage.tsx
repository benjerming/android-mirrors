import { LoginForm } from '@/features/session/components/LoginForm';

// LoginPage 表示前端登录入口页，负责承载品牌说明和会话创建表单。
export function LoginPage() {
  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(251,191,36,0.18),_transparent_35%),linear-gradient(180deg,_#fffbeb_0%,_#f5f5f4_58%,_#e7e5e4_100%)] px-6 py-10 text-stone-950 md:px-10 md:py-14">
      <div className="mx-auto flex min-h-[calc(100vh-5rem)] max-w-6xl items-center justify-center">
        <section className="w-full max-w-xl">
          <LoginForm />
        </section>
      </div>
    </main>
  );
}
