import { http, HttpResponse } from 'msw';
import molecules from '@/mocks/data/molecules.json';

const typedMolecules = molecules as any[];

export const moleculeHandlers = [
  http.get('/api/openapi/v1/molecules', ({ request }) => {
    const url = new URL(request.url);
    const page = Number(url.searchParams.get('page')) || 1;
    const pageSize = Number(url.searchParams.get('pageSize')) || 20;

    const start = (page - 1) * pageSize;
    const end = start + pageSize;
    const paginatedData = typedMolecules.slice(start, end);

    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: paginatedData,
      pagination: {
        page,
        pageSize,
        total: typedMolecules.length
      }
    });
  }),

  http.get('/api/openapi/v1/molecules/:id', ({ params }) => {
    const { id } = params;
    const molecule = typedMolecules.find((m: any) => m.id === id);

    if (!molecule) {
      return HttpResponse.json({
        code: 4004,
        message: 'Molecule not found',
        data: null
      }, { status: 404 });
    }

    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: molecule
    });
  })
];
