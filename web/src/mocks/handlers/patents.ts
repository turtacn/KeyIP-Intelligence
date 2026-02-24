import { http, HttpResponse } from 'msw';
import patents from '@/mocks/data/patents.json';

// Type assertion for mock data if needed, or rely on inference
const typedPatents = patents as any[];

export const patentHandlers = [
  http.get('/api/openapi/v1/patents', ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page')) || 1;
    const pageSize = Number(url.searchParams.get('pageSize')) || 20;
    const query = url.searchParams.get('query') || '';
    const searchType = url.searchParams.get('searchType') || 'text';

    // Simulate error
    if (url.searchParams.get('__error')) {
      return new HttpResponse(null, { status: 500, statusText: 'Internal Server Error' });
    }

    // Filter logic
    let filtered = typedPatents;

    if (searchType === 'structure' && query) {
        // Mock logic for structure search
        // Return all patents or random set to ensure "not empty" for demo
        // In real app, perform substructure search
        filtered = typedPatents;
    } else if (query) {
      const lowerQuery = query.toLowerCase();
      filtered = filtered.filter((p: any) =>
        p.title.toLowerCase().includes(lowerQuery) ||
        p.publicationNumber.toLowerCase().includes(lowerQuery) ||
        p.assignee.toLowerCase().includes(lowerQuery)
      );
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
