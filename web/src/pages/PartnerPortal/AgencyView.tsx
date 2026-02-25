import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import DataTable, { Column } from '../../components/ui/DataTable';
import StatusBadge from '../../components/ui/StatusBadge';
import { Upload, Download, Send, MessageSquare } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const AgencyView: React.FC = () => {
  const { t } = useTranslation();
  const [tasks] = useState([
    { id: 'T001', patentNo: 'CN115321456A', type: 'Draft Response', deadline: '2024-07-15', priority: 'High', status: 'In Progress' },
    { id: 'T002', patentNo: 'US20230123456A1', type: 'File Application', deadline: '2024-08-01', priority: 'Medium', status: 'Pending' },
    { id: 'T003', patentNo: 'EP3456789A1', type: 'Submit Translation', deadline: '2024-06-30', priority: 'Low', status: 'Completed' },
  ]);

  const [files] = useState([
    { name: 'Office_Action_Response_CN115.pdf', uploader: 'John Doe', date: '2024-06-20' },
    { name: 'Application_Draft_v2.docx', uploader: 'Jane Smith', date: '2024-06-18' },
  ]);

  const taskColumns: Column<any>[] = [
    { header: t('partners.agency.table.task_id'), accessor: 'id' },
    { header: t('partners.agency.table.patent_no'), accessor: 'patentNo' },
    { header: t('partners.agency.table.type'), accessor: 'type' },
    { header: t('partners.agency.table.deadline'), accessor: 'deadline' },
    {
      header: t('partners.agency.table.priority'),
      accessor: (row) => (
        <span className={`px-2 py-0.5 rounded text-xs font-bold ${
          row.priority === 'High' ? 'bg-red-100 text-red-700' :
          row.priority === 'Medium' ? 'bg-amber-100 text-amber-700' :
          'bg-green-100 text-green-700'
        }`}>
          {row.priority}
        </span>
      )
    },
    { header: t('partners.agency.table.status'), accessor: (row) => <StatusBadge status={row.status === 'Completed' ? 'completed' : row.status === 'In Progress' ? 'active' : 'pending'} label={row.status} /> },
  ];

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 h-full">
      <div className="lg:col-span-2 space-y-6">
        <Card header={t('partners.agency.tasks_title')} padding="none">
          <DataTable columns={taskColumns} data={tasks} />
        </Card>

        <Card header={t('partners.agency.files_title')} className="flex-1">
          <div className="border-2 border-dashed border-slate-300 rounded-lg p-8 text-center mb-6 hover:bg-slate-50 transition-colors cursor-pointer">
            <Upload className="w-10 h-10 text-slate-400 mx-auto mb-2" />
            <p className="text-sm text-slate-600 font-medium">{t('partners.agency.upload_hint')}</p>
            <p className="text-xs text-slate-400 mt-1">{t('partners.agency.upload_subhint')}</p>
          </div>

          <h4 className="font-semibold text-slate-800 mb-2 text-sm">{t('partners.agency.recent_files')}</h4>
          <ul className="space-y-2">
            {files.map((file, i) => (
              <li key={i} className="flex justify-between items-center p-3 bg-slate-50 rounded border border-slate-100">
                <div className="flex items-center gap-3">
                  <div className="bg-white p-2 rounded border border-slate-200">
                    <FileIcon name={file.name} />
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-700">{file.name}</p>
                    <p className="text-xs text-slate-400">Uploaded by {file.uploader} • {file.date}</p>
                  </div>
                </div>
                <Button size="sm" variant="ghost" leftIcon={<Download className="w-4 h-4" />}>
                  {t('partners.common.download')}
                </Button>
              </li>
            ))}
          </ul>
        </Card>
      </div>

      <div className="lg:col-span-1">
        <Card className="h-full flex flex-col">
          <div className="border-b border-slate-200 pb-4 mb-4">
            <h3 className="font-semibold text-slate-800 flex items-center gap-2">
              <MessageSquare className="w-4 h-4" />
              {t('partners.agency.comm_title')}
            </h3>
          </div>

          <div className="flex-1 overflow-y-auto space-y-4 pr-2 mb-4">
            {/* Mock Chat */}
            <div className="flex gap-3">
              <div className="w-8 h-8 rounded-full bg-blue-100 flex items-center justify-center text-xs font-bold text-blue-600 flex-shrink-0">JD</div>
              <div>
                <div className="bg-slate-100 rounded-lg p-3 text-sm text-slate-700">
                  Hi, could you please review the latest office action for the CN patent?
                </div>
                <div className="text-xs text-slate-400 mt-1">10:30 AM</div>
              </div>
            </div>

            <div className="flex gap-3 flex-row-reverse">
              <div className="w-8 h-8 rounded-full bg-purple-100 flex items-center justify-center text-xs font-bold text-purple-600 flex-shrink-0">AG</div>
              <div className="text-right">
                <div className="bg-blue-50 text-blue-900 rounded-lg p-3 text-sm text-left">
                  Sure, John. We received it yesterday. Our initial assessment suggests we need to narrow Claim 1 slightly. I'll upload a draft response by EOD.
                </div>
                <div className="text-xs text-slate-400 mt-1">10:45 AM</div>
              </div>
            </div>
          </div>

          <div className="mt-auto pt-4 border-t border-slate-200">
            <div className="relative">
              <input
                type="text"
                placeholder={t('partners.agency.chat_placeholder')}
                className="w-full pr-10 pl-4 py-2 border border-slate-300 rounded-full text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              />
              <button className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-blue-600 hover:bg-blue-50 rounded-full transition-colors">
                <Send className="w-4 h-4" />
              </button>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};

// Helper for file icon based on extension
const FileIcon: React.FC<{ name: string }> = ({ name }) => {
  const ext = name.split('.').pop()?.toLowerCase();
  if (ext === 'pdf') return <span className="text-red-500 font-bold text-xs">PDF</span>;
  if (ext === 'docx') return <span className="text-blue-500 font-bold text-xs">DOC</span>;
  return <span className="text-slate-500 font-bold text-xs">FILE</span>;
};

export default AgencyView;
