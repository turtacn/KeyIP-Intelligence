import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import DataTable, { Column } from '../../components/ui/DataTable';
import StatusBadge from '../../components/ui/StatusBadge';
import { Gavel, CheckCircle, Clock } from 'lucide-react';
import Modal from '../../components/ui/Modal';
import { useTranslation } from 'react-i18next';

const CounselView: React.FC = () => {
  const { t } = useTranslation();
  const [reviews, setReviews] = useState([
    { id: 'R001', reportTitle: 'FTO Analysis - Blue Emitter Gen 3', date: '2024-06-25', deadline: '2024-07-02', status: 'Pending Review' },
    { id: 'R002', reportTitle: 'Infringement Risk - Patent US11234567', date: '2024-06-22', deadline: '2024-06-30', status: 'Completed' },
  ]);

  const [selectedReview, setSelectedReview] = useState<any | null>(null);
  const [reviewNote, setReviewNote] = useState('');
  const [riskRating, setRiskRating] = useState('Acceptable');
  const [loading, setLoading] = useState(false);

  const columns: Column<any>[] = [
    { header: t('partners.counsel.table.review_id'), accessor: 'id' },
    { header: t('partners.counsel.table.report_title'), accessor: (row) => <span className="font-medium text-slate-800">{row.reportTitle}</span> },
    { header: t('partners.counsel.table.created'), accessor: 'date' },
    {
      header: t('partners.counsel.table.deadline'),
      accessor: (row) => (
        <div className="flex items-center text-amber-600 font-medium text-xs gap-1">
          <Clock className="w-3 h-3" /> {row.deadline}
        </div>
      )
    },
    { header: t('partners.counsel.table.status'), accessor: (row) => <StatusBadge status={row.status === 'Completed' ? 'completed' : 'pending'} label={row.status} /> },
    {
      header: t('partners.counsel.table.action'),
      accessor: (row) => (
        <Button
          size="sm"
          variant={row.status === 'Completed' ? 'outline' : 'primary'}
          onClick={() => setSelectedReview(row)}
          disabled={row.status === 'Completed'}
        >
          {row.status === 'Completed' ? t('partners.counsel.actions.view') : t('partners.counsel.actions.review')}
        </Button>
      )
    }
  ];

  const handleSubmitReview = () => {
    setLoading(true);
    setTimeout(() => {
      setLoading(false);
      setSelectedReview(null);
      // Mock update
      const updated = reviews.map(r => r.id === selectedReview.id ? { ...r, status: 'Completed' } : r);
      setReviews(updated);
      alert('Review submitted successfully.');
    }, 1500);
  };

  return (
    <Card header={t('partners.counsel.title')} padding="none">
      <DataTable columns={columns} data={reviews} />

      <Modal
        isOpen={!!selectedReview}
        onClose={() => setSelectedReview(null)}
        title={`Legal Review: ${selectedReview?.reportTitle}`}
        size="lg"
        footer={
          <>
            <Button variant="secondary" onClick={() => setSelectedReview(null)}>Cancel</Button>
            <Button variant="primary" onClick={handleSubmitReview} isLoading={loading} leftIcon={<CheckCircle className="w-4 h-4" />}>
              {t('partners.counsel.submit_btn')}
            </Button>
          </>
        }
      >
        <div className="space-y-6">
          <div className="bg-slate-50 p-4 rounded-lg border border-slate-200">
            <h4 className="font-semibold text-slate-800 mb-2 flex items-center gap-2">
              <Gavel className="w-4 h-4 text-slate-500" />
              Report Summary
            </h4>
            <p className="text-sm text-slate-600">
              This report analyzes the Freedom to Operate (FTO) for the proposed Blue Emitter material structure (Internal ID: BE-Gen3).
              Potential overlap detected with UDC Patent US11234567 regarding the spirobifluorene core substitution pattern.
            </p>
            <div className="mt-3 flex gap-4 text-xs font-medium text-slate-500">
              <span>Risk Score: 0.72 (High)</span>
              <span>Jurisdiction: US, EP</span>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-2">{t('partners.counsel.opinion_label')}</label>
            <textarea
              value={reviewNote}
              onChange={(e) => setReviewNote(e.target.value)}
              placeholder={t('partners.counsel.opinion_placeholder')}
              className="w-full h-40 p-3 border border-slate-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-none"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-700 mb-2">{t('partners.counsel.rating_label')}</label>
            <div className="flex gap-4">
              {['Acceptable', 'Caution', 'High Risk', 'Reject'].map((rating) => (
                <label key={rating} className={`
                  flex-1 border rounded-lg p-3 cursor-pointer transition-all text-center text-sm font-medium
                  ${riskRating === rating
                    ? 'bg-blue-50 border-blue-500 text-blue-700 ring-1 ring-blue-500'
                    : 'border-slate-200 text-slate-600 hover:bg-slate-50'
                  }
                `}>
                  <input
                    type="radio"
                    name="riskRating"
                    value={rating}
                    checked={riskRating === rating}
                    onChange={(e) => setRiskRating(e.target.value)}
                    className="sr-only"
                  />
                  {rating}
                </label>
              ))}
            </div>
          </div>
        </div>
      </Modal>
    </Card>
  );
};

export default CounselView;
