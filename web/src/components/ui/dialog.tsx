import * as React from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';

import { cn } from '@/lib/utils';

// Dialog 用来承接弹窗状态，让业务组件只关心内容和开关时机。
export const Dialog = DialogPrimitive.Root;

// DialogTrigger 表示打开弹窗的触发器，通常包住按钮或卡片入口。
export const DialogTrigger = DialogPrimitive.Trigger;

// DialogPortal 用来把弹窗内容挂到页面顶层，避免被父级布局裁切。
export const DialogPortal = DialogPrimitive.Portal;

// DialogClose 表示关闭弹窗的通用入口，给右上角关闭按钮等场景复用。
export const DialogClose = DialogPrimitive.Close;

// DialogOverlay 表示弹窗背后的遮罩层，用来弱化背景内容并承接点击关闭。
export const DialogOverlay = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Overlay>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Overlay>
>(({ className, ...props }, ref) => {
  return <DialogPrimitive.Overlay className={cn('fixed inset-0 z-50 bg-stone-950/45 backdrop-blur-sm', className)} ref={ref} {...props} />;
});

DialogOverlay.displayName = DialogPrimitive.Overlay.displayName;

// DialogContent 表示弹窗主体区域，统一处理定位、尺寸和关闭按钮外观。
export const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>
>(({ children, className, ...props }, ref) => {
  return (
    <DialogPortal>
      <DialogOverlay />
      <DialogPrimitive.Content
        className={cn(
          'fixed left-1/2 top-1/2 z-50 w-[calc(100vw-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-[2rem] border border-stone-200 bg-white p-6 shadow-2xl focus:outline-none md:p-7',
          className,
        )}
        ref={ref}
        {...props}
      >
        {children}
        <DialogPrimitive.Close className="absolute right-5 top-5 rounded-full p-2 text-stone-500 transition hover:bg-stone-100 hover:text-stone-900">
          <X className="h-4 w-4" />
          <span className="sr-only">关闭弹窗</span>
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPortal>
  );
});

DialogContent.displayName = DialogPrimitive.Content.displayName;

// DialogHeader 表示弹窗头部，通常放标题和简短说明。
export function DialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col space-y-2 text-left', className)} {...props} />;
}

// DialogFooter 表示弹窗底部操作区，在移动端会自动纵向堆叠按钮。
export function DialogFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('mt-6 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end', className)} {...props} />;
}

// DialogTitle 表示弹窗主标题，帮助用户快速理解这次操作要完成什么。
export const DialogTitle = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => {
  return <DialogPrimitive.Title className={cn('text-2xl font-semibold text-stone-950', className)} ref={ref} {...props} />;
});

DialogTitle.displayName = DialogPrimitive.Title.displayName;

// DialogDescription 表示标题下方的补充说明，用白话告诉用户这一步会影响什么。
export const DialogDescription = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => {
  return <DialogPrimitive.Description className={cn('text-sm leading-6 text-stone-600', className)} ref={ref} {...props} />;
});

DialogDescription.displayName = DialogPrimitive.Description.displayName;
