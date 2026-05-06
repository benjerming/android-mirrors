import { zodResolver } from '@hookform/resolvers/zod';
import { LoaderCircle, Plus } from 'lucide-react';
import { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useConfigOptions } from '@/features/config/api';
import { useCreateGroup } from '@/features/group/api';
import { createGroupSchema, type CreateGroupInput } from '@/features/group/schemas';
import { ApiError } from '@/lib/errors';

const HIGH_LANGUAGE_THRESHOLD = 6;

// CreateGroupDialog 承载"+ 新建分组"按钮与提交表单。
export function CreateGroupDialog() {
  const [open, setOpen] = useState(false);
  const options = useConfigOptions();
  const mutation = useCreateGroup();

  const form = useForm<CreateGroupInput>({
    resolver: zodResolver(createGroupSchema),
    defaultValues: { name: '', profileId: '', languages: [] },
  });

  const selectedLanguages = form.watch('languages');
  const showHighLanguageHint = selectedLanguages.length >= HIGH_LANGUAGE_THRESHOLD;

  function handleSubmit(values: CreateGroupInput) {
    mutation.mutate(values, {
      onSuccess: (res) => {
        if (res.failed.length > 0) {
          toast.warning(
            `${res.failed.length} 个实例创建失败：${res.failed.map((f) => f.language).join(', ')}`,
          );
        } else {
          toast.success(`分组「${res.group.name}」已创建`);
        }
        form.reset();
        setOpen(false);
      },
      onError: (err) => {
        if (err instanceof ApiError && err.status === 409) {
          form.setError('name', { message: '该分组名已被使用' });
        } else {
          toast.error(err instanceof Error ? err.message : '创建失败');
        }
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button data-testid="open-create-group">
          <Plus className="h-4 w-4" />
          新建分组
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>新建分组</DialogTitle>
        </DialogHeader>
        <Form {...form}>
          <form className="space-y-4" onSubmit={form.handleSubmit(handleSubmit)}>
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>分组名</FormLabel>
                  <FormControl>
                    <Input placeholder="例如 测试组A" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="profileId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>设备模板</FormLabel>
                  <FormControl>
                    <select
                      data-testid="profile-select"
                      className="h-11 w-full rounded-2xl border border-stone-200 bg-white px-3 text-sm"
                      {...field}
                    >
                      <option value="">请选择…</option>
                      {options.data?.profiles.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.displayName} ({p.resolution})
                        </option>
                      ))}
                    </select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <Controller
              control={form.control}
              name="languages"
              render={({ field, fieldState }) => (
                <FormItem>
                  <FormLabel>语言</FormLabel>
                  <div className="flex flex-wrap gap-2">
                    {options.data?.languages.map((l) => {
                      const checked = field.value.includes(l.code);
                      return (
                        <label
                          key={l.code}
                          className={`cursor-pointer rounded-full border px-3 py-1 text-sm ${
                            checked
                              ? 'border-amber-400 bg-amber-100 text-amber-900'
                              : 'border-stone-200 bg-white text-stone-700'
                          }`}
                        >
                          <input
                            data-testid={`lang-${l.code}`}
                            type="checkbox"
                            className="hidden"
                            checked={checked}
                            onChange={() => {
                              const next = checked
                                ? field.value.filter((c: string) => c !== l.code)
                                : [...field.value, l.code];
                              field.onChange(next);
                            }}
                          />
                          {l.label}
                        </label>
                      );
                    })}
                  </div>
                  {showHighLanguageHint ? (
                    <p data-testid="high-lang-hint" className="text-xs text-amber-700">
                      已选择 {selectedLanguages.length} 种语言，创建会同时跑较多 AVD，建议分批。
                    </p>
                  ) : null}
                  {fieldState.error ? (
                    <p className="text-xs text-rose-600">{fieldState.error.message}</p>
                  ) : null}
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button data-testid="submit-create-group" type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                创建
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
