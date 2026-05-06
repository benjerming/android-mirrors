import { LoaderCircle } from 'lucide-react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useDeleteGroup } from '@/features/group/api';

interface DeleteGroupConfirmProps {
  groupId: number;
  groupName: string;
  instanceCount: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function DeleteGroupConfirm({
  groupId,
  groupName,
  instanceCount,
  open,
  onOpenChange,
}: DeleteGroupConfirmProps) {
  const mutation = useDeleteGroup();

  function handleConfirm() {
    mutation.mutate(groupId, {
      onSuccess: () => {
        toast.success(`分组「${groupName}」已删除`);
        onOpenChange(false);
      },
      onError: (err) => {
        toast.error(err instanceof Error ? err.message : '删除失败');
      },
    });
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>删除分组「{groupName}」？</DialogTitle>
        </DialogHeader>
        <p data-testid="delete-summary" className="text-sm text-stone-600">
          这会级联删除组内 {instanceCount} 个实例及对应 AVD 文件，无法恢复。
        </p>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button
            data-testid="delete-confirm"
            type="button"
            onClick={handleConfirm}
            disabled={mutation.isPending}
            className="bg-rose-600 text-white hover:bg-rose-700"
          >
            {mutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            确认删除
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
