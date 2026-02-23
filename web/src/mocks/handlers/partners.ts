import { http, HttpResponse } from 'msw';
import companies from '../data/companies.json';

export const partnerHandlers = [
  http.get('/api/openapi/v1/partners', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: companies,
      pagination: {
        page: 1,
        pageSize: 50,
        total: companies.length
      }
    });
  })
];
