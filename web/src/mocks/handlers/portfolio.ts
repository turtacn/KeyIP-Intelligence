import { http, HttpResponse } from 'msw';
import portfolio from '@/mocks/data/portfolio.json';

const typedPortfolio = portfolio as any;

export const portfolioHandlers = [
  http.get('/api/v1/portfolio/summary', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: typedPortfolio.summary
    });
  }),

  http.get('/api/v1/portfolio/scores', () => {
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

  http.get('/api/v1/portfolio/coverage', () => {
    return HttpResponse.json({
      code: 0,
      message: 'success',
      data: typedPortfolio.coverage
    });
  }),

  http.get('/api/v1/portfolios/:id/constellation', () => {
    // Generate mock constellation data
    const domains = ['carbazole', 'triazine', 'fluorene', 'spiro', 'phosphine'];

    const points = [];
    let ptIdx = 0;
    for (let di = 0; di < domains.length; di++) {
      const ptsPerDomain = 4 + Math.floor(Math.random() * 8);
      const centerAngle = (2 * Math.PI * di) / domains.length;
      const radius = 3 + di * 1.2;
      for (let j = 0; j < ptsPerDomain; j++) {
        const angle = centerAngle + (j / ptsPerDomain) * 0.8 - 0.4;
        const r = radius + (Math.random() - 0.5) * 0.8;
        const ptType = di % 3 === 0 ? 'competitor_patent' : 'own_patent';
        points.push({
          id: `pt-${ptIdx}`,
          patent_number: `${domains[di].toUpperCase()}-${String(j + 1).padStart(4, '0')}`,
          x: Math.round(r * Math.cos(angle) * 100) / 100,
          y: Math.round(r * Math.sin(angle) * 100) / 100,
          point_type: ptType,
          assignee: ptType === 'own_patent' ? 'Our Company' : 'Competitor Corp',
          tech_domain: domains[di],
          value_score: Math.round((20 + Math.random() * 80) * 10) / 10,
          filing_year: 2019 + Math.floor(Math.random() * 6),
          legal_status: Math.random() > 0.2 ? 'granted' : 'pending',
          cluster_label: `${domains[di]}-based`,
        });
        ptIdx++;
      }
    }

    const clusters = domains.map((d, i) => {
      const centerAngle = (2 * Math.PI * i) / domains.length;
      const r = 3 + i * 1.2;
      return {
        cluster_id: `cluster-${d}`,
        label: `${d}-based`,
        center_x: Math.round(r * Math.cos(centerAngle) * 100) / 100,
        center_y: Math.round(r * Math.sin(centerAngle) * 100) / 100,
        point_count: 4 + Math.floor(Math.random() * 8),
        tech_domain: d,
      };
    });

    const whiteSpaces = [];
    for (let i = 0; i < clusters.length - 1; i++) {
      const midX = (clusters[i].center_x + clusters[i + 1].center_x) / 2;
      const midY = (clusters[i].center_y + clusters[i + 1].center_y) / 2;
      whiteSpaces.push({
        region_id: `ws-${i}`,
        center_x: Math.round(midX * 100) / 100,
        center_y: Math.round(midY * 100) / 100,
        description: `Opportunity between ${clusters[i].tech_domain} and ${clusters[i + 1].tech_domain}`,
        tech_domains: [clusters[i].tech_domain, clusters[i + 1].tech_domain],
        score: Math.round((0.8 - i * 0.12) * 100) / 100,
      });
    }

    return HttpResponse.json({
      portfolio_id: 'pf-1',
      points,
      clusters,
      white_spaces: whiteSpaces,
      total_points: points.length,
    });
  }),
];
