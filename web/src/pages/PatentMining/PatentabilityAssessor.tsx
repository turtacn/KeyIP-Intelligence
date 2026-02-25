import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { Search, Loader2, CheckCircle, AlertTriangle } from 'lucide-react';
import MoleculeViewer from '../../components/ui/MoleculeViewer';
import { useTranslation } from 'react-i18next';

const PatentabilityAssessor: React.FC = () => {
  const { t } = useTranslation();
  const [smiles, setSmiles] = useState('');
  const [description, setDescription] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any | null>(null);

  const handleAssess = () => {
    if (!smiles.trim()) return;

    setLoading(true);
    setResult(null);

    // Simulate assessment
    setTimeout(() => {
      setLoading(false);
      setResult({
        novelty: 85,
        inventiveStep: 72,
        utility: "The proposed structure shows promising improvement in EQE based on theoretical calculations, suggesting high industrial utility in OLED display panels.",
        similarPatents: [
          { id: "US11,234,567", title: "Organic Electroluminescent Device", assignee: "Samsung SDI", score: 0.82 },
          { id: "KR10-2023-001234", title: "Blue Host Material", assignee: "LG Chem", score: 0.76 },
          { id: "EP3,456,789", title: "Nitrogen-Containing Heterocyclic Derivative", assignee: "UDC", score: 0.65 }
        ],
        recommendation: "Proceed with filing"
      });
    }, 2000);
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
      {/* Input Section */}
      <Card className="flex flex-col h-full">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('mining.assessment.title')}</h3>
        <div className="space-y-4 flex-1">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">{t('mining.assessment.smiles_label')}</label>
            <textarea
              value={smiles}
              onChange={(e) => setSmiles(e.target.value)}
              placeholder={t('mining.assessment.smiles_placeholder')}
              className="w-full h-32 p-3 border border-slate-300 rounded-lg font-mono text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
            {smiles && (
              <div className="mt-2 bg-slate-50 border border-slate-200 rounded p-2">
                <p className="text-xs text-slate-500 mb-2">Structure Preview:</p>
                <MoleculeViewer smiles={smiles} width={300} height={150} />
              </div>
            )}
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">{t('mining.assessment.desc_label')}</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('mining.assessment.desc_placeholder')}
              className="w-full h-40 p-3 border border-slate-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>
        </div>
        <div className="mt-6 pt-4 border-t border-slate-100">
          <Button
            onClick={handleAssess}
            isLoading={loading}
            disabled={!smiles.trim()}
            leftIcon={<Search className="w-4 h-4" />}
            className="w-full"
          >
            {t('mining.assessment.btn_assess')}
          </Button>
        </div>
      </Card>

      {/* Result Section */}
      <Card className="flex flex-col h-full bg-slate-50/50">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('mining.assessment.report_title')}</h3>
        {loading ? (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-500">
            <Loader2 className="w-10 h-10 animate-spin text-blue-600 mb-4" />
            <p>Analyzing global patent database...</p>
            <p className="text-xs mt-2">Checking 15M+ documents</p>
          </div>
        ) : result ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
            {/* Scores */}
            <div className="grid grid-cols-2 gap-4">
              <div className="bg-white p-4 rounded-lg border border-slate-200 text-center">
                <div className="text-sm text-slate-500 mb-1">{t('mining.assessment.novelty')}</div>
                <div className="text-3xl font-bold text-green-600">{result.novelty}</div>
                <div className="w-full bg-slate-100 h-1.5 rounded-full mt-2">
                  <div className="bg-green-500 h-1.5 rounded-full" style={{ width: `${result.novelty}%` }}></div>
                </div>
              </div>
              <div className="bg-white p-4 rounded-lg border border-slate-200 text-center">
                <div className="text-sm text-slate-500 mb-1">{t('mining.assessment.inventive')}</div>
                <div className="text-3xl font-bold text-blue-600">{result.inventiveStep}</div>
                <div className="w-full bg-slate-100 h-1.5 rounded-full mt-2">
                  <div className="bg-blue-500 h-1.5 rounded-full" style={{ width: `${result.inventiveStep}%` }}></div>
                </div>
              </div>
            </div>

            {/* Utility */}
            <div className="bg-white p-4 rounded-lg border border-slate-200">
              <h4 className="font-semibold text-slate-700 mb-2 text-sm">{t('mining.assessment.utility')}</h4>
              <p className="text-sm text-slate-600 leading-relaxed">{result.utility}</p>
            </div>

            {/* Similar Patents */}
            <div>
              <h4 className="font-semibold text-slate-700 mb-2 text-sm">{t('mining.assessment.similar')}</h4>
              <div className="space-y-2">
                {result.similarPatents.map((pat: any) => (
                  <div key={pat.id} className="flex justify-between items-center bg-white p-3 rounded border border-slate-200 text-sm">
                    <div>
                      <span className="font-medium text-slate-800 mr-2">{pat.id}</span>
                      <span className="text-slate-500 truncate max-w-[150px] inline-block align-bottom">{pat.title}</span>
                    </div>
                    <div className="flex items-center text-xs">
                      <span className="text-slate-400 mr-2">{pat.assignee}</span>
                      <span className="bg-slate-100 text-slate-600 px-1.5 py-0.5 rounded font-medium">
                        {(pat.score * 100).toFixed(0)}% sim
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Recommendation */}
            <div className={`p-4 rounded-lg border flex items-center gap-3 ${
              result.novelty > 80 ? 'bg-green-50 border-green-200 text-green-800' : 'bg-amber-50 border-amber-200 text-amber-800'
            }`}>
              {result.novelty > 80 ? <CheckCircle className="w-5 h-5" /> : <AlertTriangle className="w-5 h-5" />}
              <div>
                <div className="font-bold">{t('mining.assessment.recommendation')}: {result.recommendation}</div>
                <div className="text-xs opacity-80 mt-1">Based on initial screening. Always consult a patent attorney.</div>
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 text-sm">
            <Search className="w-12 h-12 mb-4 opacity-20" />
            <p>{t('mining.assessment.enter_molecule_hint')}</p>
          </div>
        )}
      </Card>
    </div>
  );
};

export default PatentabilityAssessor;
