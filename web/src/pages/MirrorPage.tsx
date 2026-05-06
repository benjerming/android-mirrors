import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { TransformComponent, TransformWrapper } from 'react-zoom-pan-pinch';
import { toast } from 'sonner';

import { Skeleton } from '@/components/ui/skeleton';
import { useGroup } from '@/features/group/api';
import { MirrorScreen } from '@/features/mirror/components/MirrorScreen';
import { MirrorTopbar } from '@/features/mirror/components/MirrorTopbar';
import { OperationsApkSection } from '@/features/mirror/components/OperationsApkSection';
import { OperationsDrawer } from '@/features/mirror/components/OperationsDrawer';
import { OperationsFileSection } from '@/features/mirror/components/OperationsFileSection';
import { OperationsKeysSection } from '@/features/mirror/components/OperationsKeysSection';
import { useControlStore } from '@/stores/control';

export function MirrorPage() {
  const params = useParams<{ groupId?: string }>();
  const navigate = useNavigate();
  const groupIdNum = params.groupId ? Number(params.groupId) : undefined;
  const detail = useGroup(groupIdNum);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const mode = useControlStore((s) => s.mode);
  const singleTargetId = useControlStore((s) => s.singleTargetId);
  const setSingleTarget = useControlStore((s) => s.setSingleTarget);

  useEffect(() => useControlStore.getState().reset, []);

  // 独控模式下抽屉里的批量操作语义模糊（详见 spec §9.6 决策）——切到独控时强制收起。
  useEffect(() => {
    if (mode === 'single') setDrawerOpen(false);
  }, [mode]);

  // 阻止浏览器在按住 Ctrl/Cmd 时把滚轮当作页面缩放——库自身的 preventDefault
  // 只覆盖 TransformWrapper 内部，外部区域仍会触发浏览器缩放。
  useEffect(() => {
    const onWheel = (e: WheelEvent) => {
      if (e.ctrlKey || e.metaKey) e.preventDefault();
    };
    window.addEventListener('wheel', onWheel, { passive: false });
    return () => window.removeEventListener('wheel', onWheel);
  }, []);

  useEffect(() => {
    if (detail.isError) {
      toast.error('未找到该分组');
      navigate('/groups', { replace: true });
    }
  }, [detail.isError, navigate]);

  useEffect(() => {
    if (!detail.data) return;
    const hasRunning = detail.data.instances.some((i) => i.status === 'running');
    if (!hasRunning) {
      toast.warning('请先启动分组实例');
      navigate('/groups', { replace: true });
    }
  }, [detail.data, navigate]);

  if (detail.isLoading || !detail.data) {
    return (
      <section className="space-y-4">
        <Skeleton className="h-12 w-1/3" />
        <Skeleton className="h-80 w-full" />
      </section>
    );
  }

  const { group, instances } = detail.data;
  const sorted = [...instances].sort((a, b) => (a.language ?? '').localeCompare(b.language ?? ''));
  const runningIds = sorted.filter((i) => i.status === 'running').map((i) => i.id);

  // resolveTapTargets：群控 → 所有 running；独控 → 只点的那一台。
  const resolveTapTargets = (selfId: number) =>
    mode === 'single' ? [selfId] : runningIds;

  // 操作面板（按键/APK/文件）的 fanout 目标：单控时若没选则取首个 running。
  const opsTargets =
    mode === 'single'
      ? sorted.filter((i) => i.id === (singleTargetId ?? runningIds[0]) && i.status === 'running')
      : sorted.filter((i) => i.status === 'running');

  return (
    <section className="flex h-full flex-col gap-4">
      <MirrorTopbar
        groupName={group.name}
        runningCount={group.runningCount}
        instanceCount={group.instanceCount}
        onOpenDrawer={() => setDrawerOpen(true)}
      />

      <TransformWrapper
        minScale={0.5}
        maxScale={4}
        initialScale={1}
        smooth={false}
        centerOnInit
        centerZoomedOut
        wheel={{
          step: 0.1,
          activationKeys: (keys) => keys.includes('Control') || keys.includes('Meta'),
        }}
        doubleClick={{ disabled: true }}
        panning={{ activationKeys: [' '], excluded: ['video', 'button'] }}
        pinch={{ step: 5 }}
      >
        <TransformComponent
          wrapperClass="!w-full !flex-1 !min-h-0 rounded-2xl border border-stone-200 bg-stone-50"
          contentClass="!flex !w-full !min-h-full items-center justify-center p-3"
        >
          <div
            data-testid="mirror-grid"
            className="grid w-full gap-3 justify-center"
            style={{
              gridTemplateColumns: 'repeat(auto-fill, 220px)',
              // 5*220 + 4*12(gap-3) = 1148px：卡住每行最多 5 个。
              maxWidth: '1148px',
            }}
          >
            {sorted.map((inst) => (
              <MirrorScreen
                key={inst.id}
                instance={inst}
                groupId={group.id}
                resolveTapTargets={resolveTapTargets}
                highlighted={mode === 'single' && singleTargetId === inst.id}
                onSelect={(id) => {
                  if (mode === 'single') setSingleTarget(id);
                }}
              />
            ))}
          </div>
        </TransformComponent>
      </TransformWrapper>

      <OperationsDrawer open={drawerOpen} onClose={() => setDrawerOpen(false)}>
        <OperationsKeysSection targets={opsTargets} />
        <OperationsApkSection targets={opsTargets} isSingleMode={mode === 'single'} />
        <OperationsFileSection targets={opsTargets} />
      </OperationsDrawer>
    </section>
  );
}
