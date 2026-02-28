import { http, HttpResponse } from 'msw';
import patents from '@/mocks/data/patents.json';

// Type assertion for mock data if needed, or rely on inference
const typedPatents = patents as any[];

export const patentHandlers = [
  http.post('/api/v1/patents/search', async ({ request }) => {
    const body = await request.json().catch(() => ({})) as any;
    const page = Number(body.page) || 1;
    const pageSize = Number(body.page_size) || 20;
    const query = body.query || '';
    const searchType = body.query_type || 'text';

    // Simulate error
    if (query === '__error') {
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

  http.get('/api/v1/patents/:id', ({ params }) => {
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
