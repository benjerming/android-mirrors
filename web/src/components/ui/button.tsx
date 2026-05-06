import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-2xl text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-400 focus-visible:ring-offset-2',
  {
    variants: {
      variant: {
        default: 'bg-stone-950 text-white hover:bg-stone-800',
        outline: 'border border-stone-200 bg-white text-stone-900 hover:bg-stone-50',
        secondary: 'bg-amber-100 text-amber-900 hover:bg-amber-200',
      },
      size: {
        default: 'h-11 px-5 py-2.5',
        sm: 'h-9 rounded-xl px-3',
        lg: 'h-12 px-6 text-base',
        icon: 'h-10 w-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
);

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
  };

// Button 表示通用按钮外观，给登录、列表和后续操作面板共用同一套交互样式。
export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ asChild = false, className, size, variant, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button';

    return <Comp className={cn(buttonVariants({ className, size, variant }))} ref={ref} {...props} />;
  },
);

Button.displayName = 'Button';
