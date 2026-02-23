import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Badge from '../../components/ui/Badge';
import { ChevronDown, ChevronUp, Star, TrendingUp, Gavel, DollarSign } from 'lucide-react';
import { RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis, Radar, ResponsiveContainer } from 'recharts';

interface ValueScoringProps {
  patents: any[]; // Using any for simplicity as scoring fields aren't in base Patent type
}

const ValueScoring: React.FC<ValueScoringProps> = ({ patents }) => {
  const [expandedRow, setExpandedRow] = useState<string | null>(null);

  // Mock scoring data generation
  const scoredPatents = patents.map((p, index) => {
    const tech = 70 + Math.random() * 25;
    const legal = 60 + Math.random() * 30;
    const commercial = 50 + Math.random() * 40;
    const composite = (tech * 0.4 + legal * 0.3 + commercial * 0.3).toFixed(1);

    let recommendation = 'Maintain';
    if (Number(composite) > 85) recommendation = 'Accelerate';
    else if (Number(composite) > 75 && commercial > 80) recommendation = 'License';
    else if (Number(composite) < 60) recommendation = 'Abandon';

    return {
      ...p,
      rank: index + 1,
      scores: { tech, legal, commercial, composite },
      recommendation
    };
  }).sort((a, b) => Number(b.scores.composite) - Number(a.scores.composite));

  const toggleRow = (id: string) => {
    if (expandedRow === id) setExpandedRow(null);
    else setExpandedRow(id);
  };

  return (
    <Card className="flex flex-col h-full" padding="none">
      <div className="px-6 py-4 border-b border-slate-200 bg-slate-50 rounded-t-lg">
        <h3 className="font-semibold text-slate-800">Patent Value Assessment</h3>
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-slate-200">
          <thead className="bg-slate-50">
            <tr>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Rank</th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Patent No.</th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Title</th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Composite</th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">Recommendation</th>
              <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-slate-500 uppercase tracking-wider">Details</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-slate-200">
            {scoredPatents.map((patent) => (
              <React.Fragment key={patent.id}>
                <tr
                  onClick={() => toggleRow(patent.id)}
                  className={`cursor-pointer transition-colors ${expandedRow === patent.id ? 'bg-blue-50' : 'hover:bg-slate-50'}`}
                >
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-slate-900">
                    #{patent.rank}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-slate-600">
                    {patent.publicationNumber}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-slate-900 truncate max-w-[200px]" title={patent.title}>
                    {patent.title}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-bold text-slate-800">
                    {patent.scores.composite}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <Badge
                      variant={
                        patent.recommendation === 'Accelerate' ? 'success' :
                        patent.recommendation === 'Abandon' ? 'danger' :
                        patent.recommendation === 'License' ? 'info' : 'default'
                      }
                    >
                      {patent.recommendation}
                    </Badge>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                    {expandedRow === patent.id ? <ChevronUp className="w-4 h-4 ml-auto" /> : <ChevronDown className="w-4 h-4 ml-auto" />}
                  </td>
                </tr>

                {/* Expanded Details */}
                {expandedRow === patent.id && (
                  <tr className="bg-blue-50/50">
                    <td colSpan={6} className="px-6 py-4">
                      <div className="flex flex-col md:flex-row gap-8 animate-in fade-in slide-in-from-top-2 duration-200">
                        {/* Breakdown Chart */}
                        <div className="w-full md:w-1/3 h-48">
                          <ResponsiveContainer width="100%" height="100%">
                            <RadarChart cx="50%" cy="50%" outerRadius="80%" data={[
                              { subject: 'Technical', A: patent.scores.tech, fullMark: 100 },
                              { subject: 'Legal', A: patent.scores.legal, fullMark: 100 },
                              { subject: 'Commercial', A: patent.scores.commercial, fullMark: 100 },
                              { subject: 'Strategic', A: (patent.scores.tech + patent.scores.commercial)/2, fullMark: 100 },
                            ]}>
                              <PolarGrid />
                              <PolarAngleAxis dataKey="subject" />
                              <PolarRadiusAxis angle={30} domain={[0, 100]} />
                              <Radar name={patent.publicationNumber} dataKey="A" stroke="#2563eb" fill="#3b82f6" fillOpacity={0.6} />
                            </RadarChart>
                          </ResponsiveContainer>
                        </div>

                        {/* Detailed Scores */}
                        <div className="flex-1 grid grid-cols-3 gap-4">
                          <div className="bg-white p-4 rounded-lg border border-blue-100 shadow-sm text-center">
                            <div className="flex justify-center mb-2 bg-blue-100 w-8 h-8 rounded-full items-center mx-auto text-blue-600">
                              <TrendingUp className="w-4 h-4" />
                            </div>
                            <div className="text-xs text-slate-500 uppercase tracking-wide mb-1">Technical Value</div>
                            <div className="text-2xl font-bold text-slate-800">{patent.scores.tech.toFixed(0)}</div>
                            <div className="text-xs text-slate-400 mt-1">Novelty & Utility</div>
                          </div>

                          <div className="bg-white p-4 rounded-lg border border-purple-100 shadow-sm text-center">
                            <div className="flex justify-center mb-2 bg-purple-100 w-8 h-8 rounded-full items-center mx-auto text-purple-600">
                              <Gavel className="w-4 h-4" />
                            </div>
                            <div className="text-xs text-slate-500 uppercase tracking-wide mb-1">Legal Value</div>
                            <div className="text-2xl font-bold text-slate-800">{patent.scores.legal.toFixed(0)}</div>
                            <div className="text-xs text-slate-400 mt-1">Claim Strength</div>
                          </div>

                          <div className="bg-white p-4 rounded-lg border border-green-100 shadow-sm text-center">
                            <div className="flex justify-center mb-2 bg-green-100 w-8 h-8 rounded-full items-center mx-auto text-green-600">
                              <DollarSign className="w-4 h-4" />
                            </div>
                            <div className="text-xs text-slate-500 uppercase tracking-wide mb-1">Commercial Value</div>
                            <div className="text-2xl font-bold text-slate-800">{patent.scores.commercial.toFixed(0)}</div>
                            <div className="text-xs text-slate-400 mt-1">Market Size</div>
                          </div>
                        </div>
                      </div>
                    </td>
                  </tr>
                )}
              </React.Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
};

export default ValueScoring;
