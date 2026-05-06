import { zodResolver } from '@hookform/resolvers/zod';
import { LoaderCircle } from 'lucide-react';
import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { useRenameGroup } from '@/features/group/api';
import { renameGroupSchema, type RenameGroupInput } from '@/features/group/schemas';
import { ApiError } from '@/lib/errors';

interface RenameGroupDialogProps {
  groupId: number;
  currentName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function RenameGroupDialog({ groupId, currentName, open, onOpenChange }: RenameGroupDialogProps) {
  const mutation = useRenameGroup();
  const form = useForm<RenameGroupInput>({
    resolver: zodResolver(renameGroupSchema),
    defaultValues: { name: currentName },
  });

  useEffect(() => {
    if (open) form.reset({ name: currentName });
  }, [open, currentName, form]);

  function handleSubmit(values: RenameGroupInput) {
    mutation.mutate(
      { id: groupId, name: values.name },
      {
        onSuccess: () => {
          toast.success(`已重命名为「${values.name}」`);
          onOpenChange(false);
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 409) {
            form.setError('name', { message: '该分组名已被使用' });
          } else {
            toast.error(err instanceof Error ? err.message : '重命名失败');
          }
        },
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>重命名分组</DialogTitle>
        </DialogHeader>
        <Form {...form}>
          <form className="space-y-4" onSubmit={form.handleSubmit(handleSubmit)}>
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>新分组名</FormLabel>
                  <FormControl>
                    <Input data-testid="rename-input" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                取消
              </Button>
              <Button data-testid="rename-submit" type="submit" disabled={mutation.isPending}>
                {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                保存
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
