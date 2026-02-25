import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import StatusBadge from '../../components/ui/StatusBadge';
import Button from '../../components/ui/Button';
import Modal from '../../components/ui/Modal';
import { Company } from '../../types/domain';
import { UserPlus, Edit, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface AdminViewProps {
  partners: Company[];
  loading: boolean;
}

const AdminView: React.FC<AdminViewProps> = ({ partners, loading }) => {
  const { t } = useTranslation();
  const [showAddModal, setShowAddModal] = useState(false);
  const [newPartner, setNewPartner] = useState<Partial<Company>>({ name: '', country: 'US', type: 'Agency' });

  const columns: Column<Company>[] = [
    { header: t('partners.admin.table.name'), accessor: 'name' },
    { header: t('partners.admin.table.type'), accessor: 'type' },
    { header: t('partners.admin.table.country'), accessor: 'country' },
    { header: t('partners.admin.table.projects'), accessor: () => Math.floor(Math.random() * 10) }, // Mock
    { header: t('partners.admin.table.performance'), accessor: () => <span className="text-green-600 font-bold">{(85 + Math.random() * 15).toFixed(0)}%</span> },
    { header: t('partners.admin.table.status'), accessor: () => <StatusBadge status="active" /> },
    {
      header: t('partners.admin.table.actions'),
      accessor: () => (
        <div className="flex space-x-2">
          <Button size="sm" variant="ghost"><Edit className="w-4 h-4" /></Button>
          <Button size="sm" variant="ghost" className="text-red-600 hover:text-red-700 hover:bg-red-50"><Trash2 className="w-4 h-4" /></Button>
        </div>
      )
    }
  ];

  const handleAddPartner = () => {
    // In real app, call API
    setShowAddModal(false);
    alert('Partner added (mock)');
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold text-slate-800">{t('partners.admin.title')}</h2>
        <Button onClick={() => setShowAddModal(true)} leftIcon={<UserPlus className="w-4 h-4" />}>
          {t('partners.admin.add_btn')}
        </Button>
      </div>

      <Card padding="none">
        <DataTable columns={columns} data={partners} isLoading={loading} />
      </Card>

      <Modal
        isOpen={showAddModal}
        onClose={() => setShowAddModal(false)}
        title="Add New Partner"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Organization Name</label>
            <input
              type="text"
              className="w-full border-slate-300 rounded-lg text-sm"
              value={newPartner.name}
              onChange={(e) => setNewPartner({...newPartner, name: e.target.value})}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Type</label>
              <select
                className="w-full border-slate-300 rounded-lg text-sm"
                value={newPartner.type}
                onChange={(e) => setNewPartner({...newPartner, type: e.target.value})}
              >
                <option value="Agency">Patent Agency</option>
                <option value="LawFirm">Law Firm</option>
                <option value="Research">Research Institute</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-slate-700 mb-1">Country</label>
              <select
                className="w-full border-slate-300 rounded-lg text-sm"
                value={newPartner.country}
                onChange={(e) => setNewPartner({...newPartner, country: e.target.value})}
              >
                <option value="US">United States</option>
                <option value="CN">China</option>
                <option value="EP">Europe</option>
                <option value="JP">Japan</option>
                <option value="KR">Korea</option>
              </select>
            </div>
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-3">
          <Button variant="secondary" onClick={() => setShowAddModal(false)}>Cancel</Button>
          <Button variant="primary" onClick={handleAddPartner}>Create Partner</Button>
        </div>
      </Modal>
    </div>
  );
};

export default AdminView;
