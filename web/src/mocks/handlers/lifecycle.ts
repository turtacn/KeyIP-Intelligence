import { http, HttpResponse } from 'msw';
import lifecycle from '@/mocks/data/lifecycle.json';

const typedLifecycle = lifecycle as any[];

export const lifecycleHandlers = [
  http.get('/api/v1/lifecycle/events', ({ request }) => {
    const url = new URL(request.url);
    const jurisdiction = url.searchParams.get('jurisdiction');
    const status = url.searchParams.get('status');

    let filtered = typedLifecycle;
    if (jurisdiction) filtered = filtered.filter((e: any) => e.jurisdiction === jurisdiction);
    if (status) filtered = filtered.filter((e: any) => e.status === status);

    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: filtered,
      pagination: {
        page: 1,
        pageSize: 50,
        total: filtered.length
      }
    });
  })
];
