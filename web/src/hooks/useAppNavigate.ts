import { useNavigate } from 'react-router-dom';

// useAppNavigate 表示对路由跳转做一层轻封装，后面补埋点或统一参数时不需要满仓替换。
export function useAppNavigate() {
  return useNavigate();
}
