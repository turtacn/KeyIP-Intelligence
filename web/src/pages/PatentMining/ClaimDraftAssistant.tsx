import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { PenTool, Copy, Download, Loader2, Check } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const ClaimDraftAssistant: React.FC = () => {
  const { t } = useTranslation();
  const [description, setDescription] = useState('');
  const [features, setFeatures] = useState('');
  const [loading, setLoading] = useState(false);
  const [draft, setDraft] = useState<string[]>([]);
  const [copied, setCopied] = useState(false);

  const handleGenerate = () => {
    if (!description.trim() || !features.trim()) return;
    setLoading(true);
    setCopied(false);
    setTimeout(() => {
      setLoading(false);
      setDraft([
        "1. An organic light-emitting device comprising: a first electrode; a second electrode; and an organic layer disposed between the first electrode and the second electrode, wherein the organic layer comprises a compound represented by Formula 1.",
        "2. The organic light-emitting device of claim 1, wherein the organic layer is an emission layer.",
        "3. The organic light-emitting device of claim 1, wherein the compound of Formula 1 is a blue dopant.",
        "4. The organic light-emitting device of claim 1, wherein the first electrode is an anode and the second electrode is a cathode."
      ]);
    }, 2500);
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(draft.join('\n\n'));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleExport = () => {
    const element = document.createElement("a");
    const file = new Blob([draft.join('\n\n')], {type: 'text/plain'});
    element.href = URL.createObjectURL(file);
    element.download = "draft_claims.txt";
    document.body.appendChild(element);
    element.click();
  };

  const updateClaim = (index: number, newText: string) => {
    const newDraft = [...draft];
    newDraft[index] = newText;
    setDraft(newDraft);
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
      <Card className="flex flex-col h-full">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">Invention Scope</h3>
        <div className="space-y-4 flex-1">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Detailed Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('mining.draft.desc_placeholder')}
              className="w-full h-48 p-3 border border-slate-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Key Differentiating Features (comma-separated)</label>
            <input
              type="text"
              value={features}
              onChange={(e) => setFeatures(e.target.value)}
              placeholder={t('mining.draft.keywords_placeholder')}
              className="w-full p-3 border border-slate-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>
        </div>
        <div className="mt-6 pt-4 border-t border-slate-100">
          <Button
            onClick={handleGenerate}
            isLoading={loading}
            disabled={!description.trim() || !features.trim()}
            className="w-full"
            leftIcon={<PenTool className="w-4 h-4" />}
          >
            Generate Claim Draft
          </Button>
        </div>
      </Card>

      <Card className="flex flex-col h-full bg-slate-50/50">
        <div className="flex justify-between items-center mb-4">
          <h3 className="text-lg font-semibold text-slate-800">Draft Claims</h3>
          {draft.length > 0 && (
            <div className="flex space-x-2">
              <Button size="sm" variant="secondary" onClick={handleCopy} leftIcon={copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}>
                {copied ? 'Copied' : 'Copy'}
              </Button>
              <Button size="sm" variant="outline" onClick={handleExport} leftIcon={<Download className="w-4 h-4" />}>
                Export
              </Button>
            </div>
          )}
        </div>

        {loading ? (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-500">
            <Loader2 className="w-10 h-10 animate-spin text-blue-600 mb-4" />
            <p>Synthesizing legal language...</p>
          </div>
        ) : draft.length > 0 ? (
          <div className="flex-1 overflow-y-auto space-y-4 pr-2">
            {draft.map((claim, index) => (
              <div key={index} className="bg-white p-3 rounded-lg border border-slate-200 shadow-sm animate-in fade-in slide-in-from-bottom-2 duration-300" style={{ animationDelay: `${index * 100}ms` }}>
                <div className="text-xs font-bold text-slate-400 mb-1 uppercase tracking-wider">
                  {index === 0 ? 'Independent Claim' : 'Dependent Claim'}
                </div>
                <textarea
                  value={claim}
                  onChange={(e) => updateClaim(index, e.target.value)}
                  className="w-full h-24 p-2 text-sm font-serif leading-relaxed border-none focus:ring-0 resize-y bg-transparent"
                />
              </div>
            ))}
          </div>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 text-sm">
            <PenTool className="w-12 h-12 mb-4 opacity-20" />
            <p>Enter details to generate patent claims.</p>
          </div>
        )}
      </Card>
    </div>
  );
};

export default ClaimDraftAssistant;
