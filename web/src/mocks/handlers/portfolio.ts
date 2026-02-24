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
        // Return categoryScores instead of summary scores if needed by Treemap
        // But ValueScoring component might use 'scores' (summary).
        // Let's create a NEW endpoint for category scores if needed,
        // OR simply return typedPortfolio.categoryScores if the frontend expects it.
        // Wait, CoverageTreemap uses 'scores' prop passed from index.tsx:
        // const scoresData = (scores.data || {}) ...
        // If we change this endpoint to return categoryScores, it might break ValueScoring?
        // Let's check usage in index.tsx.
        // ValueScoring uses 'patents' (list). CoverageTreemap uses 'scoresData'.
        // So 'scores' endpoint is likely intended for portfolio-wide scores, not categories.
        // However, CoverageTreemap expects category-based scores.
        // So let's merge them or return categoryScores for a new endpoint.
        // Simpler: Just return categoryScores here for now as the prompt implies "scores" prop for Treemap comes from this hook.
        data: typedPortfolio.categoryScores || typedPortfolio.scores
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
