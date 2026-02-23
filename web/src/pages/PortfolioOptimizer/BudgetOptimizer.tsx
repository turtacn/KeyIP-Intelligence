import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import Modal from '../../components/ui/Modal';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { AlertOctagon, TrendingDown } from 'lucide-react';

const BudgetOptimizer: React.FC = () => {
  const [showConfirm, setShowConfirm] = useState(false);
  const [optimizing, setOptimizing] = useState(false);

  // Mock data
  const costData = [
    { name: 'CN', cost: 12000 },
    { name: 'US', cost: 25000 },
    { name: 'EP', cost: 18000 },
    { name: 'JP', cost: 8000 },
    { name: 'KR', cost: 5000 },
  ];

  const recommendations = [
    { id: 'US7654321', cost: 3500, score: 42, reason: "High maintenance cost, low strategic value (Score: 42)", action: "Abandon" },
    { id: 'JP20185678', cost: 1200, score: 55, reason: "Duplicate coverage in better jurisdiction", action: "Review" },
    { id: 'EP1234567', cost: 2800, score: 38, reason: "Technology obsolete", action: "Abandon" },
  ];

  const totalSavings = recommendations.reduce((acc, r) => acc + r.cost, 0);

  const handleApply = () => {
    setOptimizing(true);
    setTimeout(() => {
      setOptimizing(false);
      setShowConfirm(false);
      window.alert('Optimization recommendations applied successfully. Updated portfolio status.');
    }, 2000);
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
      <Card header="Annual Maintenance Cost by Jurisdiction" className="h-full">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={costData}
            margin={{ top: 20, right: 30, left: 20, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" vertical={false} />
            <XAxis dataKey="name" />
            <YAxis />
            <Tooltip cursor={{ fill: '#f8fafc' }} />
            <Legend />
            <Bar dataKey="cost" fill="#3b82f6" radius={[4, 4, 0, 0]} name="Annual Cost (USD)" />
          </BarChart>
        </ResponsiveContainer>
      </Card>

      <Card header="Optimization Recommendations" className="h-full flex flex-col">
        <div className="bg-green-50 p-4 rounded-lg border border-green-100 mb-4 flex items-center justify-between">
          <div>
            <h4 className="font-semibold text-green-800">Potential Annual Savings</h4>
            <p className="text-sm text-green-600">Based on value-to-cost analysis</p>
          </div>
          <div className="text-2xl font-bold text-green-700">
            ${totalSavings.toLocaleString()}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto space-y-3 pr-2">
          {recommendations.map((rec) => (
            <div key={rec.id} className="flex justify-between items-center p-3 border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <span className="font-mono font-medium text-slate-700">{rec.id}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full font-bold ${rec.action === 'Abandon' ? 'bg-red-100 text-red-700' : 'bg-amber-100 text-amber-700'}`}>
                    {rec.action}
                  </span>
                </div>
                <div className="text-xs text-slate-500">{rec.reason}</div>
              </div>
              <div className="text-right">
                <div className="font-bold text-slate-700">${rec.cost.toLocaleString()}</div>
                <div className="text-xs text-slate-400">/ year</div>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-4 pt-4 border-t border-slate-100">
          <Button
            onClick={() => setShowConfirm(true)}
            className="w-full"
            variant="primary"
            leftIcon={<TrendingDown className="w-4 h-4" />}
          >
            Apply Optimization Plan
          </Button>
        </div>
      </Card>

      <Modal
        isOpen={showConfirm}
        onClose={() => setShowConfirm(false)}
        title="Confirm Portfolio Optimization"
        size="md"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowConfirm(false)}>Cancel</Button>
            <Button variant="danger" onClick={handleApply} isLoading={optimizing}>Confirm & Apply</Button>
          </>
        }
      >
        <div className="space-y-4 text-center py-4">
          <div className="mx-auto w-12 h-12 bg-red-100 rounded-full flex items-center justify-center mb-4">
            <AlertOctagon className="w-6 h-6 text-red-600" />
          </div>
          <p className="text-slate-700 font-medium">
            This action will mark {recommendations.filter(r => r.action === 'Abandon').length} patents for abandonment.
          </p>
          <p className="text-sm text-slate-500">
            Once confirmed, instructions will be sent to external counsel to stop annuity payments. This action cannot be easily undone after the deadline passes.
          </p>
        </div>
      </Modal>
    </div>
  );
};

export default BudgetOptimizer;
