import { http, HttpResponse } from 'msw';
import auth from '@/mocks/data/auth.json';

export const authHandlers = [
  // POST /api/v1/auth/signin — mock 模式下绕过真实数据库，返回预签名 JWT
  http.post('/api/v1/auth/signin', async ({ request }) => {
    const body = await request.json() as Record<string, string> | null;
    if (!body?.email || !body?.password) {
      return HttpResponse.json({ code: 'BadRequest', message: 'email and password are required' }, { status: 400 });
    }
    // 接受所有合法格式的凭据（mock 模式不做真正验证）
    if (body.email.includes('@') && body.password.length >= 6) {
      return HttpResponse.json(auth.signin);
    }
    return HttpResponse.json({ code: 'Unauthorized', message: '[COMMON_003] invalid credentials' }, { status: 401 });
  }),

  // GET /api/v1/auth/me — 返回当前登录用户信息
  http.get('/api/v1/auth/me', ({ request }) => {
    const authHeader = request.headers.get('Authorization');
    if (!authHeader?.startsWith('Bearer ')) {
      return HttpResponse.json({ code: 'Unauthorized', message: 'authorization header required' }, { status: 401 });
    }
    return HttpResponse.json(auth.me);
  }),
];
