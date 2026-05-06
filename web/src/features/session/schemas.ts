import { z } from 'zod';

// loginSchema 表示登录表单的输入规则，先只要求用户名非空，方便后端用最小参数完成会话创建。
export const loginSchema = z.object({
  username: z.string().trim().min(1, '请输入用户名').max(64, '用户名不能超过 64 个字符'),
});

// LoginFormValues 表示登录表单提交时的字段结构，给表单和请求层共用同一份约束。
export type LoginFormValues = z.infer<typeof loginSchema>;
