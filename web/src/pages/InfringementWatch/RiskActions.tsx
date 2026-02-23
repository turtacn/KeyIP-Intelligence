import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import Modal from '../../components/ui/Modal';
import { InfringementAlert } from '../../types/domain';
import { FileText, Edit, UserPlus, CheckCircle } from 'lucide-react';

interface RiskActionsProps {
  alert: InfringementAlert;
  onMarkReviewed: (id: string) => void;
}

const RiskActions: React.FC<RiskActionsProps> = ({ alert, onMarkReviewed }) => {
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const [showDesignModal, setShowDesignModal] = useState(false);

  const handleGenerateReport = () => {
    setLoadingAction('report');
    setTimeout(() => {
      setLoadingAction(null);
      // Simulate download
      const element = document.createElement("a");
      const file = new Blob(["FTO Report Mock Content..."], {type: 'text/plain'});
      element.href = URL.createObjectURL(file);
      element.download = `FTO_Report_${alert.targetPatentId}.txt`;
      document.body.appendChild(element); // Required for this to work in FireFox
      element.click();
    }, 2000);
  };

  const handleDesignAround = () => {
    setShowDesignModal(true);
  };

  const handleAssign = () => {
    setLoadingAction('assign');
    setTimeout(() => {
      setLoadingAction(null);
      window.alert('Assigned to Legal Team successfully.');
    }, 1500);
  };

  const handleMarkReviewed = () => {
    setLoadingAction('review');
    setTimeout(() => {
      setLoadingAction(null);
      onMarkReviewed(alert.id);
    }, 1000);
  };

  return (
    <>
      <Card className="mt-6 border-t-4 border-t-slate-200">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">Risk Mitigation Actions</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          <Button
            variant="outline"
            onClick={handleGenerateReport}
            isLoading={loadingAction === 'report'}
            leftIcon={<FileText className="w-4 h-4" />}
          >
            Generate FTO Report
          </Button>

          <Button
            variant="secondary"
            onClick={handleDesignAround}
            leftIcon={<Edit className="w-4 h-4" />}
          >
            Design Around
          </Button>

          <Button
            variant="secondary"
            onClick={handleAssign}
            isLoading={loadingAction === 'assign'}
            leftIcon={<UserPlus className="w-4 h-4" />}
          >
            Assign to Legal
          </Button>

          <Button
            variant="primary"
            onClick={handleMarkReviewed}
            isLoading={loadingAction === 'review'}
            leftIcon={<CheckCircle className="w-4 h-4" />}
            disabled={alert.status === 'reviewed'}
          >
            {alert.status === 'reviewed' ? 'Reviewed' : 'Mark as Reviewed'}
          </Button>
        </div>
      </Card>

      <Modal
        isOpen={showDesignModal}
        onClose={() => setShowDesignModal(false)}
        title="Design Around Suggestions"
        size="lg"
      >
        <div className="space-y-4">
          <p className="text-slate-600 mb-4">
            AI-generated structural modifications to avoid infringement while maintaining key properties.
          </p>
          {[1, 2, 3].map((i) => (
            <div key={i} className="border border-slate-200 rounded-lg p-4 hover:bg-slate-50 transition-colors">
              <div className="flex justify-between items-center mb-2">
                <span className="font-semibold text-slate-800">Option {i}</span>
                <span className="text-xs bg-green-100 text-green-700 px-2 py-1 rounded-full">
                  Risk Score: {(0.2 * i).toFixed(2)} (Low)
                </span>
              </div>
              <div className="bg-white p-2 rounded border border-slate-200 font-mono text-xs text-slate-500 mb-2">
                {`C1=CC=C(C=C1)N(C2=CC=CC=C2)C3=CC=C(C=C3)C4=CC=C(C=C4)N(C5=CC=CC=C5)C6=CC=CC=C6.Modification${i}`}
              </div>
              <div className="flex justify-between text-xs text-slate-500">
                <span>Predicted EQE: {(20 - i).toFixed(1)}%</span>
                <span>Predicted Lifetime: {(95 + i * 2).toFixed(0)}h</span>
              </div>
            </div>
          ))}
        </div>
      </Modal>
    </>
  );
};

export default RiskActions;
