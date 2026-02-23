import { http, HttpResponse } from 'msw';
import companies from '../data/companies.json';

const typedCompanies = companies as any[];

export const partnerHandlers = [
  http.get('/api/openapi/v1/partners', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: typedCompanies,
      pagination: {
        page: 1,
        pageSize: 50,
        total: typedCompanies.length
      }
    });
  })
];
