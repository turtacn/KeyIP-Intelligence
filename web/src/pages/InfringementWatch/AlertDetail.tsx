import React, { useEffect, useState } from 'react';
import Card from '../../components/ui/Card';
import StatusBadge from '../../components/ui/StatusBadge';
import { InfringementAlert, Patent } from '../../types/domain';
import { patentService } from '../../services/patent.service';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import { ExternalLink, Calendar, User } from 'lucide-react';

interface AlertDetailProps {
  alert: InfringementAlert;
}

const AlertDetail: React.FC<AlertDetailProps> = ({ alert }) => {
  const [patent, setPatent] = useState<Patent | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchPatent = async () => {
      setLoading(true);
      try {
        const response = await patentService.getPatentById(alert.targetPatentId);
        setPatent(response.data);
      } catch (err) {
        console.error('Failed to fetch patent details', err);
      } finally {
        setLoading(false);
      }
    };

    if (alert.targetPatentId) {
      fetchPatent();
    }
  }, [alert.targetPatentId]);

  if (loading) {
    return <div className="h-64 flex items-center justify-center"><LoadingSpinner /></div>;
  }

  if (!patent) {
    return <div className="p-8 text-center text-slate-500">Patent details not found.</div>;
  }

  return (
    <Card className="mb-6 border-l-4 border-l-red-500 shadow-sm">
      <div className="flex justify-between items-start mb-4">
        <div>
          <h2 className="text-xl font-bold text-slate-900 flex items-center gap-2">
            {patent.publicationNumber}
            <a href={`https://patents.google.com/patent/${patent.publicationNumber}`} target="_blank" rel="noopener noreferrer" className="text-slate-400 hover:text-blue-600 transition-colors">
              <ExternalLink className="w-4 h-4" />
            </a>
          </h2>
          <h3 className="text-lg text-slate-700 font-medium mt-1">{patent.title}</h3>
        </div>
        <div className="flex flex-col items-end gap-2">
          <StatusBadge status={patent.legalStatus === 'granted' ? 'active' : 'pending'} label={patent.legalStatus} />
          <span className="text-xs text-slate-500 bg-slate-100 px-2 py-1 rounded">
            {patent.ipcCodes[0]}
          </span>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 text-sm text-slate-600 mb-6">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <User className="w-4 h-4 text-slate-400" />
            <span className="font-medium text-slate-900">Assignee:</span> {patent.assignee}
          </div>
          <div className="flex items-center gap-2">
            <Calendar className="w-4 h-4 text-slate-400" />
            <span className="font-medium text-slate-900">Filing Date:</span> {patent.filingDate}
          </div>
        </div>
        <div className="bg-slate-50 p-3 rounded-lg border border-slate-100">
          <h4 className="font-semibold text-slate-800 mb-2">Risk Assessment</h4>
          <div className="flex justify-between items-center mb-1">
            <span>Literal Infringement:</span>
            <span className="font-mono font-medium text-red-600">{(alert.literalScore * 100).toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-200 rounded-full h-1.5 mb-3">
            <div className="bg-red-500 h-1.5 rounded-full" style={{ width: `${alert.literalScore * 100}%` }}></div>
          </div>

          <div className="flex justify-between items-center mb-1">
            <span>Doctrine of Equivalents:</span>
            <span className="font-mono font-medium text-amber-600">{(alert.docScore! * 100).toFixed(1)}%</span>
          </div>
          <div className="w-full bg-slate-200 rounded-full h-1.5">
            <div className="bg-amber-500 h-1.5 rounded-full" style={{ width: `${alert.docScore! * 100}%` }}></div>
          </div>
        </div>
      </div>

      <div>
        <h4 className="font-semibold text-slate-800 mb-2">Abstract</h4>
        <p className="text-slate-600 text-sm leading-relaxed bg-slate-50 p-4 rounded-lg border border-slate-100">
          {patent.abstract}
        </p>
      </div>
    </Card>
  );
};

export default AlertDetail;
