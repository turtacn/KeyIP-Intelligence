import { http, HttpResponse } from 'msw';
import dashboard from '../data/dashboard.json';

export const dashboardHandlers = [
  http.get('/api/openapi/v1/dashboard/metrics', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: dashboard
    });
  })
];
