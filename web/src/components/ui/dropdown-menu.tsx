import * as React from 'react';
import * as DropdownMenuPrimitive from '@radix-ui/react-dropdown-menu';

import { cn } from '@/lib/utils';

// DropdownMenu 用来承接轻量级悬浮菜单状态，适合展示选择摘要或快捷操作。
export const DropdownMenu = DropdownMenuPrimitive.Root;

// DropdownMenuTrigger 表示打开菜单的触发入口，通常包住一个按钮。
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;

// DropdownMenuContent 表示菜单浮层主体，统一处理背景、边框和阴影。
export const DropdownMenuContent = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Content>
>(({ className, sideOffset = 8, ...props }, ref) => {
  return (
    <DropdownMenuPrimitive.Portal>
      <DropdownMenuPrimitive.Content
        className={cn('z-50 min-w-56 rounded-2xl border border-stone-200 bg-white p-2 shadow-xl', className)}
        ref={ref}
        sideOffset={sideOffset}
        {...props}
      />
    </DropdownMenuPrimitive.Portal>
  );
});

DropdownMenuContent.displayName = DropdownMenuPrimitive.Content.displayName;

// DropdownMenuLabel 表示菜单分组标题，用来解释下面一组内容的意义。
export const DropdownMenuLabel = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Label>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Label>
>(({ className, ...props }, ref) => {
  return <DropdownMenuPrimitive.Label className={cn('px-3 py-2 text-xs font-semibold uppercase tracking-[0.22em] text-stone-500', className)} ref={ref} {...props} />;
});

DropdownMenuLabel.displayName = DropdownMenuPrimitive.Label.displayName;

// DropdownMenuItem 表示菜单里的单条项目，支持键盘和鼠标统一交互。
export const DropdownMenuItem = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Item>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Item>
>(({ className, ...props }, ref) => {
  return (
    <DropdownMenuPrimitive.Item
      className={cn(
        'flex cursor-default select-none items-center rounded-xl px-3 py-2 text-sm text-stone-700 outline-none transition focus:bg-stone-100 focus:text-stone-950',
        className,
      )}
      ref={ref}
      {...props}
    />
  );
});

DropdownMenuItem.displayName = DropdownMenuPrimitive.Item.displayName;

// DropdownMenuSeparator 表示菜单里的分隔线，帮助用户更快区分信息分组。
export const DropdownMenuSeparator = React.forwardRef<
  React.ElementRef<typeof DropdownMenuPrimitive.Separator>,
  React.ComponentPropsWithoutRef<typeof DropdownMenuPrimitive.Separator>
>(({ className, ...props }, ref) => {
  return <DropdownMenuPrimitive.Separator className={cn('my-1 h-px bg-stone-200', className)} ref={ref} {...props} />;
});

DropdownMenuSeparator.displayName = DropdownMenuPrimitive.Separator.displayName;
