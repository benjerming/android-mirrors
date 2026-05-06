import { Boxes, Plus } from 'lucide-react';

import { Button } from '@/components/ui/button';

type EmptyInstancesProps = {
  onCreateClick: () => void;
};

// EmptyInstances 表示用户还没有任何实例时的空状态，引导他先去创建第一台机器。
export function EmptyInstances({ onCreateClick }: EmptyInstancesProps) {
  return (
    <section className="rounded-[2rem] border border-dashed border-stone-300 bg-[linear-gradient(135deg,_rgba(251,191,36,0.16),_transparent_45%),linear-gradient(180deg,_#fffbeb_0%,_#fafaf9_100%)] p-8 text-center md:p-12">
      <div className="mx-auto max-w-xl space-y-4">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-white text-amber-600 shadow-sm">
          <Boxes className="h-8 w-8" />
        </div>
        <div className="space-y-2">
          <h2 className="text-2xl font-semibold text-stone-950">还没有可控制的实例</h2>
          <p className="text-sm leading-6 text-stone-600">
            先从一个模板创建实例，后面才能做多选进入镜像、文件推送和独控操作。
          </p>
        </div>
        <Button onClick={onCreateClick} size="lg" type="button">
          <Plus className="h-4 w-4" />
          创建第一台实例
        </Button>
      </div>
    </section>
  );
}
