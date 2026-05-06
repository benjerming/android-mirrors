import { zodResolver } from '@hookform/resolvers/zod';
import { LoaderCircle, PlusCircle } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import type { CreateInstanceInput, TemplateDTO } from '@/features/instance/api';
import { useCreateInstanceMutation } from '@/features/instance/hooks';
import { cn } from '@/lib/utils';

const createInstanceSchema = z.object({
  templateId: z.coerce.number().int().positive('请选择模板'),
  tag: z.string().trim().min(1, '请输入实例标签'),
  mode: z.enum(['reusable', 'ephemeral', 'debug']),
});

type CreateInstanceFormValues = z.infer<typeof createInstanceSchema>;

type CreateInstanceDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  templates: TemplateDTO[];
};

const MODE_OPTIONS: Array<{ value: CreateInstanceInput['mode']; label: string; description: string }> = [
  { value: 'reusable', label: '可复用', description: '退出登录后保留实例，只做停机。' },
  { value: 'ephemeral', label: '临时机', description: '适合短时任务，后续容易回收。' },
  { value: 'debug', label: '调试机', description: '适合排查问题，便于单独保留环境。' },
];

// CreateInstanceDialog 表示实例新建弹窗，负责收集模板、标签和保留模式。
export function CreateInstanceDialog({ open, onOpenChange, templates }: CreateInstanceDialogProps) {
  const mutation = useCreateInstanceMutation();
  const form = useForm<CreateInstanceFormValues>({
    resolver: zodResolver(createInstanceSchema),
    defaultValues: {
      templateId: templates[0]?.id ?? 0,
      tag: '',
      mode: 'reusable',
    },
  });

  useEffect(() => {
    if (!open) {
      form.reset({
        templateId: templates[0]?.id ?? 0,
        tag: '',
        mode: 'reusable',
      });
      return;
    }

    if (templates[0] && !form.getValues('templateId')) {
      form.setValue('templateId', templates[0].id);
    }
  }, [form, open, templates]);

  // handleSubmit 用来触发建机请求，成功后顺便关掉弹窗并清空旧输入。
  function handleSubmit(values: CreateInstanceFormValues) {
    mutation.mutate(values, {
      onSuccess: () => {
        onOpenChange(false);
      },
    });
  }

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>创建实例</DialogTitle>
          <DialogDescription>选择模板并填写标签，系统会按你的选项生成一台新的 Android 实例。</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form className="mt-6 space-y-5" onSubmit={form.handleSubmit(handleSubmit)}>
            <FormField
              control={form.control}
              name="templateId"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>模板</FormLabel>
                  <FormControl>
                    <select
                      className="flex h-11 w-full rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-900 shadow-sm outline-none transition focus-visible:border-amber-400 focus-visible:ring-2 focus-visible:ring-amber-200"
                      disabled={templates.length === 0 || mutation.isPending}
                      value={field.value || ''}
                      onChange={(event) => field.onChange(Number(event.target.value))}
                    >
                      {templates.length === 0 ? <option value="">暂无模板可用</option> : null}
                      {templates.map((template) => (
                        <option key={template.id} value={template.id}>
                          {template.description ? `${template.name} — ${template.description}` : template.name}
                        </option>
                      ))}
                    </select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="tag"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>实例标签</FormLabel>
                  <FormControl>
                    <Input disabled={mutation.isPending} placeholder="例如 值守-01" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <fieldset className="space-y-3">
              <Label>保留模式</Label>
              <FormField
                control={form.control}
                name="mode"
                render={({ field }) => (
                  <FormItem className="space-y-3">
                    <FormControl>
                      <div className="grid gap-3 md:grid-cols-3">
                        {MODE_OPTIONS.map((option) => (
                          <button
                            className={cn(
                              'rounded-[1.5rem] border px-4 py-4 text-left transition',
                              field.value === option.value
                                ? 'border-amber-400 bg-amber-50 text-amber-950 ring-2 ring-amber-200'
                                : 'border-stone-200 bg-stone-50 text-stone-700 hover:border-stone-300 hover:bg-white',
                            )}
                            key={option.value}
                            onClick={() => field.onChange(option.value)}
                            type="button"
                          >
                            <div className="flex items-center gap-2 text-sm font-semibold">
                              <PlusCircle className="h-4 w-4" />
                              {option.label}
                            </div>
                            <p className="mt-2 text-xs leading-5 text-current/80">{option.description}</p>
                          </button>
                        ))}
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </fieldset>

            <DialogFooter>
              <Button disabled={mutation.isPending} onClick={() => onOpenChange(false)} type="button" variant="outline">
                取消
              </Button>
              <Button disabled={mutation.isPending || templates.length === 0} type="submit">
                {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                创建实例
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
