import { http, HttpResponse } from 'msw';
import dashboard from '@/mocks/data/dashboard.json';

export const dashboardHandlers = [
  http.get('/api/v1/dashboard/metrics', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: dashboard
    });
  })
];
