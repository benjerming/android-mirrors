import * as React from 'react';
import * as LabelPrimitive from '@radix-ui/react-label';

import { cn } from '@/lib/utils';

// Label 表示输入框上方的文字说明，统一字重和字段间距。
export const Label = React.forwardRef<
  React.ElementRef<typeof LabelPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof LabelPrimitive.Root>
>(({ className, ...props }, ref) => {
  return <LabelPrimitive.Root className={cn('text-sm font-medium text-stone-700', className)} ref={ref} {...props} />;
});

Label.displayName = LabelPrimitive.Root.displayName;
