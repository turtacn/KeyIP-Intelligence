import { http, HttpResponse } from 'msw';
import portfolio from '@/mocks/data/portfolio.json';

const typedPortfolio = portfolio as any;

export const portfolioHandlers = [
  http.get('/api/openapi/v1/portfolio/summary', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: typedPortfolio.summary
    });
  }),

  http.get('/api/openapi/v1/portfolio/scores', () => {
      return HttpResponse.json({
        code: 0,
        message: 'success',
        data: typedPortfolio.scores
      });
    }),

  http.get('/api/openapi/v1/portfolio/coverage', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: typedPortfolio.coverage
    });
  })
];
