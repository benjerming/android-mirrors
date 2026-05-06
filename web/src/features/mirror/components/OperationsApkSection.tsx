import { LoaderCircle, Upload } from 'lucide-react';
import { useRef } from 'react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { useApkHistory, useUploadApk } from '@/features/artifact/api';
import { useInstallApp } from '@/features/app/api';
import type { GroupInstance } from '@/generated/types';

interface OperationsApkSectionProps {
  targets: GroupInstance[];
  isSingleMode: boolean;
}

// OperationsApkSection 实现 spec §9.6 的 APK 区：上传历史 + 上传后链式安装。
export function OperationsApkSection({ targets, isSingleMode }: OperationsApkSectionProps) {
  const fileInput = useRef<HTMLInputElement>(null);
  const history = useApkHistory();
  const uploadMutation = useUploadApk();
  const installMutation = useInstallApp();

  async function handleFile(file: File) {
    try {
      const artifact = await uploadMutation.mutateAsync(file);
      await fanInstall(artifact.id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : '上传失败');
    }
  }

  async function fanInstall(artifactId: number) {
    const results = await Promise.allSettled(
      targets.map((inst) => installMutation.mutateAsync({ instanceId: inst.id, artifactId })),
    );
    const ok = results.filter((r) => r.status === 'fulfilled').length;
    const allLocaleApplied = results.every(
      (r) => r.status === 'fulfilled' && (r.value as { localeApplied: boolean }).localeApplied,
    );
    const localeText = allLocaleApplied ? '并设置语言' : '';
    if (ok > 0) {
      toast.success(
        isSingleMode ? `已安装到 1 台${localeText}` : `已安装到 ${ok} 台${localeText}`,
      );
    }
    const failed = results.length - ok;
    if (failed > 0) {
      toast.warning(`${failed} 台安装失败`);
    }
  }

  return (
    <section className="space-y-3">
      <h4 className="text-sm font-semibold text-stone-900">APK 安装</h4>
      <input
        ref={fileInput}
        type="file"
        accept=".apk"
        className="hidden"
        data-testid="apk-file-input"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void handleFile(file);
        }}
      />
      <Button
        size="sm"
        onClick={() => fileInput.current?.click()}
        disabled={uploadMutation.isPending || installMutation.isPending}
      >
        {uploadMutation.isPending || installMutation.isPending ? (
          <LoaderCircle className="h-4 w-4 animate-spin" />
        ) : (
          <Upload className="h-4 w-4" />
        )}
        选择 APK 上传并安装
      </Button>

      <div className="space-y-1">
        <p className="text-xs text-stone-500">最近上传</p>
        {history.isLoading ? (
          <p className="text-xs text-stone-400">加载中…</p>
        ) : (history.data ?? []).length === 0 ? (
          <p className="text-xs text-stone-400">暂无</p>
        ) : (
          <ul data-testid="apk-history" className="space-y-1">
            {(history.data ?? []).map((a) => (
              <li
                key={a.id}
                className="flex items-center justify-between rounded-xl border border-stone-200 px-3 py-2 text-xs"
              >
                <span className="truncate text-stone-700">{a.originName}</span>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-6 px-2 text-[11px]"
                  onClick={() => void fanInstall(a.id)}
                >
                  安装
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
