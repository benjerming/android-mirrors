import * as React from 'react';
import * as TooltipPrimitive from '@radix-ui/react-tooltip';

import { cn } from '@/lib/utils';

// TooltipProvider 用来给一组提示层提供统一延迟，避免鼠标划过时频繁闪动。
export const TooltipProvider = TooltipPrimitive.Provider;

// Tooltip 表示单个提示层的开关状态，通常包住一个需要说明的控件。
export const Tooltip = TooltipPrimitive.Root;

// TooltipTrigger 表示触发提示层的元素，可以是按钮、图标或文字。
export const TooltipTrigger = TooltipPrimitive.Trigger;

// TooltipContent 表示浮出的说明内容，用来解释按钮为什么不可用或当前状态代表什么。
export const TooltipContent = React.forwardRef<
  React.ElementRef<typeof TooltipPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>
>(({ className, sideOffset = 8, ...props }, ref) => {
  return (
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Content
        className={cn('z-50 max-w-xs rounded-2xl bg-stone-950 px-3 py-2 text-sm leading-5 text-white shadow-xl', className)}
        ref={ref}
        sideOffset={sideOffset}
        {...props}
      />
    </TooltipPrimitive.Portal>
  );
});

TooltipContent.displayName = TooltipPrimitive.Content.displayName;
