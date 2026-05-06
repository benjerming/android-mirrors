// ApiError 表示接口虽然返回了响应，但结果不是我们期望的成功状态。
export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message || '请求失败');
    this.name = 'ApiError';
    this.status = status;
  }
}

// UnauthorizedError 表示当前身份已失效，触发统一的登录清理逻辑。
export class UnauthorizedError extends ApiError {
  constructor(message = '登录状态已失效') {
    super(message, 401);
    this.name = 'UnauthorizedError';
  }
}

// ServerError 表示后端 5xx，调用方一般只需提示"稍后重试"。
export class ServerError extends ApiError {
  constructor(message: string, status: number) {
    super(message || '服务暂时不可用', status);
    this.name = 'ServerError';
  }
}

// NetworkError 表示请求根本没到后端（断网、CORS、DNS 失败等）。
export class NetworkError extends Error {
  constructor(message = '网络异常，请检查连接') {
    super(message);
    this.name = 'NetworkError';
  }
}
