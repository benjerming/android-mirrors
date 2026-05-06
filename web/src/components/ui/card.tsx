import * as React from 'react';

import { cn } from '@/lib/utils';

// Card 表示带边框和阴影的信息卡片，适合承载登录区块或后续业务模块。
export const Card = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(({ className, ...props }, ref) => {
  return <div className={cn('rounded-[2rem] border bg-white text-stone-950', className)} ref={ref} {...props} />;
});

Card.displayName = 'Card';

// CardHeader 表示卡片顶部区域，通常放标题、副标题和图标。
export const CardHeader = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return <div className={cn('flex flex-col p-8', className)} ref={ref} {...props} />;
  },
);

CardHeader.displayName = 'CardHeader';

// CardTitle 表示卡片主标题，给使用者稳定的字号和字重。
export const CardTitle = React.forwardRef<HTMLHeadingElement, React.HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => {
    return <h2 className={cn('text-2xl font-semibold tracking-tight text-stone-950', className)} ref={ref} {...props} />;
  },
);

CardTitle.displayName = 'CardTitle';

// CardDescription 表示卡片补充说明，用来承接次一级信息。
export const CardDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => {
    return <p className={cn('text-sm leading-6 text-stone-600', className)} ref={ref} {...props} />;
  },
);

CardDescription.displayName = 'CardDescription';

// CardContent 表示卡片正文区域，通常放表单和核心交互元素。
export const CardContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => {
    return <div className={cn('px-8 pb-8', className)} ref={ref} {...props} />;
  },
);

CardContent.displayName = 'CardContent';
