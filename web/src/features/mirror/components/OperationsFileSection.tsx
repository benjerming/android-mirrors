import { LoaderCircle, Upload } from 'lucide-react';
import { useRef, useState } from 'react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useDeleteFile, usePushFile } from '@/features/file/api';
import type { GroupInstance } from '@/generated/types';

interface OperationsFileSectionProps {
  targets: GroupInstance[];
}

// OperationsFileSection 实现 spec §9.6 的文件区：批量推送 / 删除。
export function OperationsFileSection({ targets }: OperationsFileSectionProps) {
  const fileInput = useRef<HTMLInputElement>(null);
  const [remotePath, setRemotePath] = useState('/sdcard/Download/');
  const pushMutation = usePushFile();
  const deleteMutation = useDeleteFile();

  async function handleFile(file: File) {
    if (!remotePath.trim()) {
      toast.error('请填写设备目录');
      return;
    }
    const results = await Promise.allSettled(
      targets.map((inst) =>
        pushMutation.mutateAsync({ instanceId: inst.id, file, remotePath }),
      ),
    );
    const ok = results.filter((r) => r.status === 'fulfilled').length;
    if (ok > 0) toast.success(`已推送到 ${ok} 台`);
    if (ok < results.length) toast.warning(`${results.length - ok} 台推送失败`);
  }

  async function handleDelete() {
    if (!remotePath.trim()) return;
    const results = await Promise.allSettled(
      targets.map((inst) => deleteMutation.mutateAsync({ instanceId: inst.id, remotePath })),
    );
    const ok = results.filter((r) => r.status === 'fulfilled').length;
    if (ok > 0) toast.success(`已删除 ${ok} 台对应路径`);
    if (ok < results.length) toast.warning(`${results.length - ok} 台删除失败`);
  }

  const busy = pushMutation.isPending || deleteMutation.isPending;

  return (
    <section className="space-y-2">
      <h4 className="text-sm font-semibold text-stone-900">文件</h4>
      <Input
        data-testid="remote-path-input"
        value={remotePath}
        onChange={(e) => setRemotePath(e.target.value)}
        placeholder="设备路径 /sdcard/Download/"
      />
      <input
        ref={fileInput}
        type="file"
        className="hidden"
        data-testid="file-input"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void handleFile(file);
        }}
      />
      <div className="flex flex-wrap gap-2">
        <Button size="sm" disabled={busy} onClick={() => fileInput.current?.click()}>
          {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
          推送文件
        </Button>
        <Button size="sm" variant="outline" disabled={busy} onClick={() => void handleDelete()}>
          删除该路径
        </Button>
      </div>
    </section>
  );
}
