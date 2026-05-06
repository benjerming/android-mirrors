import { zodResolver } from '@hookform/resolvers/zod';
import { AlertCircle, LoaderCircle, LogIn } from 'lucide-react';
import { useForm } from 'react-hook-form';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useLoginMutation } from '@/features/session/hooks';
import { loginSchema, type LoginFormValues } from '@/features/session/schemas';
import {
  getStoredUsernameHistory,
  getStoredUsernameLastUsed,
  storeUsernameAfterLogin,
} from '@/features/session/storage';
import { ApiError } from '@/lib/errors';

const USERNAME_HISTORY_DATALIST_ID = 'username-history-options';

// LoginForm 表示登录页真正的表单区，负责收集用户名、展示错误并触发登录请求。
export function LoginForm() {
  const usernameHistory = getStoredUsernameHistory();
  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      username: getStoredUsernameLastUsed(),
    },
  });
  const mutation = useLoginMutation();

  // handleSubmit 用来把成功登录后的本地用户名历史也一起更新，方便下次快速选择。
  function handleSubmit(values: LoginFormValues) {
    mutation.mutate(values, {
      onSuccess: () => {
        storeUsernameAfterLogin(values.username);
      },
    });
  }

  return (
    <Card className="border-stone-200/80 bg-white/95 shadow-xl shadow-amber-950/5">
      <CardHeader className="space-y-3">
        <div className="space-y-1">
          <CardTitle>登录</CardTitle>
          <CardDescription>输入用户名即可自动注册和登录。</CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form className="space-y-5" onSubmit={form.handleSubmit(handleSubmit)}>
            <FormField
              control={form.control}
              name="username"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>用户名</FormLabel>
                  <FormControl>
                    <Input autoComplete="username" list={USERNAME_HISTORY_DATALIST_ID} placeholder="例如 atlas" {...field} />
                  </FormControl>
                  <datalist id={USERNAME_HISTORY_DATALIST_ID}>
                    {usernameHistory.map((username) => (
                      <option key={username} value={username} />
                    ))}
                  </datalist>
                  <FormMessage />
                </FormItem>
              )}
            />

            {mutation.isError ? (
              <div className="flex items-start gap-2 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
                <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
                <p>{mutation.error instanceof ApiError ? mutation.error.message || '登录失败，请稍后重试。' : '登录失败，请稍后重试。'}</p>
              </div>
            ) : null}

            <Button className="w-full" disabled={mutation.isPending} type="submit">
              {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <LogIn className="h-4 w-4" />}
              开始使用
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  );
}
