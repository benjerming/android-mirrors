import * as React from 'react';

import { cn } from '@/lib/utils';

// Input 表示通用输入框样式，统一处理边框、聚焦态和占位文字风格。
export const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<'input'>>(({ className, ...props }, ref) => {
  return (
    <input
      className={cn(
        'flex h-11 w-full rounded-2xl border border-stone-200 bg-white px-4 py-2 text-sm text-stone-900 shadow-sm outline-none transition placeholder:text-stone-400 focus-visible:border-amber-400 focus-visible:ring-2 focus-visible:ring-amber-200 disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      ref={ref}
      {...props}
    />
  );
});

Input.displayName = 'Input';
