import React from 'react';
import Card from '../../components/ui/Card';
import Badge from '../../components/ui/Badge';
import { Claim, InfringementAlert } from '../../types/domain';
import { useTranslation } from 'react-i18next';

interface ClaimAnalysisProps {
  claims?: Claim[];
  alert: InfringementAlert;
}

const ClaimAnalysis: React.FC<ClaimAnalysisProps> = ({ claims = [], alert }) => {
  const { t } = useTranslation();

  // Mock analysis logic for display
  const relevantClaims = claims.length > 0 ? claims : [
    {
      id: 'mock_1',
      patentId: alert.targetPatentId,
      type: 'independent',
      text: '1. An organic electroluminescent compound represented by Formula 1, wherein R1 and R2 are independently selected from a substituted or unsubstituted C6-C30 aryl group...',
      elements: ['organic electroluminescent compound', 'Formula 1', 'R1 aryl group', 'R2 aryl group']
    },
    {
      id: 'mock_2',
      patentId: alert.targetPatentId,
      type: 'dependent',
      text: '2. The compound of claim 1, wherein R1 is a phenyl group or a naphthyl group.',
      elements: ['R1 phenyl', 'R1 naphthyl']
    }
  ];

  return (
    <Card className="mb-6">
      <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('infringement.claims.title')}</h3>

      <div className="space-y-4">
        {relevantClaims.map((claim, index) => {
          // Mock match percentage based on alert score
          const matchPercent = index === 0 ? (alert.literalScore * 100).toFixed(0) : (alert.literalScore * 80).toFixed(0);

          return (
            <div key={claim.id} className="border border-slate-200 rounded-lg overflow-hidden">
              <div className="bg-slate-50 px-4 py-2 border-b border-slate-200 flex justify-between items-center">
                <div className="flex items-center gap-3">
                  <span className="font-bold text-slate-700">{t('infringement.claims.claim_label')} {index + 1}</span>
                  <Badge variant={claim.type === 'independent' ? 'info' : 'default'} size="sm">
                    {claim.type}
                  </Badge>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-slate-500">{t('infringement.claims.match')}:</span>
                  <span className={`font-bold ${Number(matchPercent) > 80 ? 'text-red-600' : 'text-amber-600'}`}>
                    {matchPercent}%
                  </span>
                </div>
              </div>
              <div className="p-4 bg-white">
                <p className="text-sm text-slate-700 leading-relaxed font-mono">
                  {/* Simple highlight simulation */}
                  {claim.text.split(' ').map((word, i) => {
                     // Randomly highlight some words for visual effect if match is high
                     const shouldHighlight = Number(matchPercent) > 50 && Math.random() > 0.7;
                     return (
                       <span key={i} className={shouldHighlight ? "bg-yellow-100 text-yellow-800 rounded px-0.5" : ""}>
                         {word}{' '}
                       </span>
                     );
                  })}
                </p>
                {claim.elements && (
                  <div className="mt-3 pt-3 border-t border-slate-100 flex flex-wrap gap-2">
                    {claim.elements.map((el, i) => (
                      <span key={i} className="text-xs bg-slate-100 text-slate-600 px-2 py-1 rounded-full border border-slate-200">
                        {el}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
};

export default ClaimAnalysis;
