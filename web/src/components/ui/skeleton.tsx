import { cn } from '@/lib/utils';

// Skeleton 表示数据还没回来时的占位块，后续列表页和镜像页加载态会复用它。
export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('animate-pulse rounded-2xl bg-stone-200/80', className)} {...props} />;
}
