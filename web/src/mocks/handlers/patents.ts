import { http, HttpResponse } from 'msw';
import patents from '@/mocks/data/patents.json';

// Type assertion for mock data if needed, or rely on inference
const typedPatents = patents as any[];

export const patentHandlers = [
  http.get('/api/openapi/v1/patents', ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page')) || 1;
    const pageSize = Number(url.searchParams.get('pageSize')) || 20;

    // Simulate error
    if (url.searchParams.get('__error')) {
      return new HttpResponse(null, { status: 500, statusText: 'Internal Server Error' });
    }

    const start = (page - 1) * pageSize;
    const end = start + pageSize;
    const paginatedData = typedPatents.slice(start, end);

    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: paginatedData,
      pagination: {
        page,
        pageSize,
        total: typedPatents.length
      }
    });
  }),

  http.get('/api/openapi/v1/patents/:id', ({ params }) => {
    const { id } = params;
    const patent = typedPatents.find((p: any) => p.id === id);

    if (!patent) {
      return HttpResponse.json({
        code: 4004,
        message: 'Patent not found',
        data: null
      }, { status: 404 });
    }

    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: patent
    });
  })
];
