import type { GroupSummary } from '@/generated/types';
import { GroupRow } from '@/features/group/components/GroupRow';

interface GroupListProps {
  groups: GroupSummary[];
}

// GroupList 表示已经渲染好的分组数组；空状态由 GroupsPage 负责。
export function GroupList({ groups }: GroupListProps) {
  return (
    <div className="space-y-3">
      {groups.map((g) => (
        <GroupRow key={g.id} group={g} />
      ))}
    </div>
  );
}
