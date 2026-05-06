import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

// cn 用来把多个 className 合并成最终结果，后续 shadcn 组件会频繁依赖它。
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
