import { describe, expect, it } from 'vitest';

import { createGroupSchema, renameGroupSchema } from '@/features/group/schemas';

describe('createGroupSchema', () => {
  it('校验合法分组名 / profile / 至少一个语言', () => {
    const r = createGroupSchema.safeParse({
      name: '测试组A',
      profileId: 'medium_phone',
      languages: ['zh-CN'],
    });
    expect(r.success).toBe(true);
  });

  it('拒绝空 languages', () => {
    const r = createGroupSchema.safeParse({
      name: '测试组A',
      profileId: 'medium_phone',
      languages: [],
    });
    expect(r.success).toBe(false);
  });

  it('拒绝空格 / 非法字符', () => {
    expect(
      createGroupSchema.safeParse({
        name: '组 A',
        profileId: 'medium_phone',
        languages: ['zh-CN'],
      }).success,
    ).toBe(false);
  });

  it('拒绝超过 32 字符', () => {
    expect(
      createGroupSchema.safeParse({
        name: 'a'.repeat(33),
        profileId: 'medium_phone',
        languages: ['zh-CN'],
      }).success,
    ).toBe(false);
  });
});

describe('renameGroupSchema', () => {
  it('使用相同的 name 规则', () => {
    expect(renameGroupSchema.safeParse({ name: '新名字' }).success).toBe(true);
    expect(renameGroupSchema.safeParse({ name: '' }).success).toBe(false);
  });
});
