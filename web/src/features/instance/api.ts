import { apiClient } from '@/lib/apiClient';

// InstanceMode 表示前端当前支持的实例保留模式，创建弹窗会让用户从这几种里选择。
export type InstanceMode = 'reusable' | 'ephemeral' | 'debug';

// InstanceStatus 表示实例当前运行状态，实例页会根据它决定能不能进入镜像。
export type InstanceStatus = 'running' | 'stopped';

// InstanceDTO 表示实例列表接口返回给前端的最小数据结构。
export type InstanceDTO = {
  id: number;
  name: string;
  tag: string;
  mode: InstanceMode;
  status: InstanceStatus;
  templateId: number;
};

// TemplateDTO 表示新建弹窗要展示的模板信息。
export type TemplateDTO = {
  id: number;
  name: string;
  description: string;
  systemImage: string;
};

// CreateInstanceInput 表示前端新建实例时要提交给后端的字段集合。
export type CreateInstanceInput = {
  templateId: number;
  tag: string;
  mode: InstanceMode;
};

// listInstances 用来读取当前用户的实例列表，给实例页主卡片区提供真实数据。
export async function listInstances() {
  return apiClient<InstanceDTO[]>('/api/v1/instances');
}

// listTemplates 用来读取可选模板列表，给新建实例弹窗填充下拉选项。
export async function listTemplates() {
  return apiClient<TemplateDTO[]>('/api/v1/templates');
}

// createInstance 用来按模板、标签和保留模式创建一台新实例。
export async function createInstance(input: CreateInstanceInput) {
  return apiClient<InstanceDTO>('/api/v1/instances', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

// deleteInstance 用来删除当前用户名下的某台实例，主要给列表页“删除实例”按钮使用。
export async function deleteInstance(id: number) {
  return apiClient<{ success: boolean }>(`/api/v1/instances/${id}`, {
    method: 'DELETE',
  });
}

// startInstance / stopInstance 给镜像页副屏的单实例启停 icon 用。
export async function startInstance(id: number) {
  return apiClient<{ success: boolean }>(`/api/v1/instances/${id}/start`, {
    method: 'POST',
  });
}

export async function stopInstance(id: number) {
  return apiClient<{ success: boolean }>(`/api/v1/instances/${id}/stop`, {
    method: 'POST',
  });
}
