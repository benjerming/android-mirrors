import { z } from 'zod';

// NAME_RE 与后端 internal/domain/group.nameRe 保持一致：中文 / 字母 / 数字 / 下划线 / 中划线。
const NAME_RE = /^[一-鿿㐀-䶿A-Za-z0-9_\-]+$/;

export const createGroupSchema = z.object({
  name: z
    .string()
    .min(1, '请输入分组名')
    .max(32, '分组名最多 32 个字符')
    .regex(NAME_RE, '只能含中文/字母/数字/下划线/中划线'),
  profileId: z.string().min(1, '请选择设备模板'),
  languages: z.array(z.string()).min(1, '至少选择一种语言'),
});
export type CreateGroupInput = z.infer<typeof createGroupSchema>;

export const renameGroupSchema = z.object({
  name: createGroupSchema.shape.name,
});
export type RenameGroupInput = z.infer<typeof renameGroupSchema>;
