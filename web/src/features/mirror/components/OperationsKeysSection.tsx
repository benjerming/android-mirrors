import { Button } from '@/components/ui/button';
import type { GroupInstance } from '@/generated/types';
import { apiClient } from '@/lib/apiClient';

interface OperationsKeysSectionProps {
  targets: GroupInstance[];
}

const SPECIAL_KEYS = [
  { label: 'HOME', endpoint: 'home' },
  { label: 'BACK', endpoint: 'back' },
];

// OperationsKeysSection 提供 HOME / BACK 等特殊按键，按当前模式 fan-out。
export function OperationsKeysSection({ targets }: OperationsKeysSectionProps) {
  async function send(endpoint: string) {
    await Promise.allSettled(
      targets.map((inst) =>
        apiClient(`/api/v1/instances/${inst.id}/control/${endpoint}`, { method: 'POST' }),
      ),
    );
  }

  return (
    <section className="space-y-2">
      <h4 className="text-sm font-semibold text-stone-900">特殊按键</h4>
      <div className="flex flex-wrap gap-2">
        {SPECIAL_KEYS.map((k) => (
          <Button key={k.endpoint} size="sm" variant="outline" onClick={() => send(k.endpoint)}>
            {k.label}
          </Button>
        ))}
      </div>
    </section>
  );
}
