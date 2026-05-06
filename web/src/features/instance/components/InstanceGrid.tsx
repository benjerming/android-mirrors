import type { InstanceDTO } from '@/features/instance/api';
import { InstanceCard } from '@/features/instance/components/InstanceCard';

type InstanceGridProps = {
  instances: InstanceDTO[];
  selectedIds: number[];
  deletingId: number | null;
  onToggle: (id: number) => void;
  onDelete: (instance: InstanceDTO) => void;
};

// InstanceGrid 表示实例卡片网格，负责把列表数据平铺成桌面优先的卡片布局。
export function InstanceGrid({ instances, selectedIds, deletingId, onToggle, onDelete }: InstanceGridProps) {
  return (
    <div className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
      {instances.map((instance) => (
        <InstanceCard
          instance={instance}
          isDeleting={deletingId === instance.id}
          key={instance.id}
          onDelete={onDelete}
          onToggle={onToggle}
          selected={selectedIds.includes(instance.id)}
        />
      ))}
    </div>
  );
}
