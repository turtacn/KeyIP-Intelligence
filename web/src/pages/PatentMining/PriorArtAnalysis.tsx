import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { Search, Loader2, FileText, CheckCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const PriorArtAnalysis: React.FC = () => {
  const { t } = useTranslation();
  const [claimText, setClaimText] = useState('');
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<any | null>(null);

  const handleAnalyze = () => {
    if (!claimText.trim()) return;
    setLoading(true);
    setReport(null);
    setTimeout(() => {
      setLoading(false);
      setReport({
        noveltyAssessment: "The claimed invention appears to have novel elements, specifically the use of a deuterated spirobifluorene core, which is not disclosed in the cited prior art.",
        priorArt: [
          { pubNum: "US20210123456A1", score: 0.88, overlap: ["Spirobifluorene core", "Blue emission"], diff: "Lack of deuteration" },
          { pubNum: "KR1020200098765A", score: 0.75, overlap: ["OLED Device structure", "HTL material"], diff: "Different substituent on C-7 position" },
          { pubNum: "JP2019154321A", score: 0.62, overlap: ["General formula matches"], diff: "Specific R1/R2 combinations not disclosed" },
        ]
      });
    }, 2000);
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
      <Card className="flex flex-col h-full">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('mining.prior_art.input_title')}</h3>
        <div className="flex-1">
          <textarea
            value={claimText}
            onChange={(e) => setClaimText(e.target.value)}
            placeholder={t('mining.prior_art.claim_placeholder')}
            className="w-full h-full min-h-[300px] p-4 border border-slate-300 rounded-lg text-sm font-mono focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
          />
        </div>
        <div className="mt-4 pt-4 border-t border-slate-100">
          <Button
            onClick={handleAnalyze}
            isLoading={loading}
            disabled={!claimText.trim()}
            className="w-full"
            leftIcon={<Search className="w-4 h-4" />}
          >
            {t('mining.prior_art.btn_analyze')}
          </Button>
        </div>
      </Card>

      <Card className="flex flex-col h-full bg-slate-50/50 overflow-hidden">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('mining.prior_art.report_title')}</h3>
        {loading ? (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-500">
            <Loader2 className="w-10 h-10 animate-spin text-blue-600 mb-4" />
            <p>{t('mining.prior_art.scanning')}</p>
          </div>
        ) : report ? (
          <div className="flex-1 overflow-y-auto pr-2 space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="bg-white p-4 rounded-lg border border-slate-200 shadow-sm">
              <h4 className="font-semibold text-slate-700 mb-2 flex items-center">
                <CheckCircle className="w-4 h-4 text-green-600 mr-2" />
                {t('mining.prior_art.overall_assessment')}
              </h4>
              <p className="text-sm text-slate-600 leading-relaxed">{report.noveltyAssessment}</p>
            </div>

            <div>
              <h4 className="font-semibold text-slate-700 mb-3 text-sm">{t('mining.prior_art.closest_docs')}</h4>
              <div className="space-y-3">
                {report.priorArt.map((doc: any, i: number) => (
                  <div key={i} className="bg-white p-4 rounded-lg border border-slate-200 shadow-sm hover:shadow-md transition-shadow">
                    <div className="flex justify-between items-start mb-2">
                      <span className="font-mono font-medium text-blue-600">{doc.pubNum}</span>
                      <span className={`px-2 py-0.5 rounded text-xs font-bold ${doc.score > 0.8 ? 'bg-red-100 text-red-700' : 'bg-amber-100 text-amber-700'}`}>
                        {(doc.score * 100).toFixed(0)}% {t('mining.prior_art.match')}
                      </span>
                    </div>
                    <div className="space-y-2 text-xs">
                      <div className="flex gap-2">
                        <span className="font-medium text-slate-500 w-16 flex-shrink-0">{t('mining.prior_art.overlaps')}:</span>
                        <div className="flex flex-wrap gap-1">
                          {doc.overlap.map((o: string, j: number) => (
                            <span key={j} className="bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded border border-slate-200">{o}</span>
                          ))}
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <span className="font-medium text-slate-500 w-16 flex-shrink-0">{t('mining.prior_art.differs')}:</span>
                        <span className="text-green-700 bg-green-50 px-1.5 py-0.5 rounded border border-green-100 flex-1">{doc.diff}</span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 text-sm">
            <FileText className="w-12 h-12 mb-4 opacity-20" />
            <p>{t('mining.prior_art.empty_state')}</p>
          </div>
        )}
      </Card>
    </div>
  );
};

export default PriorArtAnalysis;
