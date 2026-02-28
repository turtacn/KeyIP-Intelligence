import { http, HttpResponse } from 'msw';
import alerts from '@/mocks/data/alerts.json';

const typedAlerts = alerts as any[];

export const infringementHandlers = [
  http.get('/api/v1/alerts', ({ request }) => {
    const url = new URL(request.url);
    const riskLevel = url.searchParams.get('riskLevel');
    const page = Number(url.searchParams.get('page')) || 1;
    const pageSize = Number(url.searchParams.get('pageSize')) || 20;

    let filtered = typedAlerts;
    if (riskLevel) {
      filtered = filtered.filter((a: any) => a.riskLevel === riskLevel);
    }

    const start = (page - 1) * pageSize;
    const end = start + pageSize;
    const paginatedData = filtered.slice(start, end);

    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: paginatedData,
      pagination: {
        page,
        pageSize,
        total: filtered.length
      }
    });
  })
];
