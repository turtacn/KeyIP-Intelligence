import { http, HttpResponse } from 'msw';
import portfolio from '../data/portfolio.json';

export const portfolioHandlers = [
  http.get('/api/openapi/v1/portfolio/summary', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: portfolio.summary
    });
  }),

  http.get('/api/openapi/v1/portfolio/scores', () => {
      return HttpResponse.json({
        code: 0,
        message: 'success',
        data: portfolio.scores
      });
    }),

  http.get('/api/openapi/v1/portfolio/coverage', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: portfolio.coverage
    });
  })
];
