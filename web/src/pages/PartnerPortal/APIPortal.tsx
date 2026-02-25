import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { Copy, Terminal, Key, Shield } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const APIPortal: React.FC = () => {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const apiKey = 'sk_test_51Mz...Xy9z'; // Mock

  const handleCopyKey = () => {
    navigator.clipboard.writeText(apiKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const endpoints = [
    { method: 'GET', path: '/patents', desc: 'List patents with pagination' },
    { method: 'POST', path: '/patents/search', desc: 'Advanced search with filters' },
    { method: 'POST', path: '/molecules/check', desc: 'Check molecule patentability' },
    { method: 'GET', path: '/alerts', desc: 'Get active infringement alerts' },
  ];

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 h-full">
      <div className="lg:col-span-2 space-y-6">
        <Card header={t('partners.api.doc_title')} padding="none">
          <div className="p-6 border-b border-slate-100">
            <h3 className="font-semibold text-slate-800 mb-2">{t('partners.api.intro_title')}</h3>
            <p className="text-sm text-slate-600 mb-4">
              {t('partners.api.intro_desc')}
            </p>
            <div className="bg-slate-900 text-slate-300 p-4 rounded-lg font-mono text-xs overflow-x-auto">
              <span className="text-purple-400">curl</span> -X GET https://api.keyip.com/v1/patents \<br/>
              &nbsp;&nbsp;-H <span className="text-green-400">"Authorization: Bearer {apiKey}"</span>
            </div>
          </div>

          <div className="p-6">
            <h3 className="font-semibold text-slate-800 mb-4">{t('partners.api.endpoints_title')}</h3>
            <div className="space-y-3">
              {endpoints.map((ep, i) => (
                <div key={i} className="flex items-center gap-4 p-3 border border-slate-200 rounded-lg hover:bg-slate-50 transition-colors">
                  <span className={`
                    w-16 text-center text-xs font-bold py-1 rounded
                    ${ep.method === 'GET' ? 'bg-blue-100 text-blue-700' : 'bg-green-100 text-green-700'}
                  `}>
                    {ep.method}
                  </span>
                  <span className="font-mono text-sm text-slate-700 flex-1">{ep.path}</span>
                  <span className="text-sm text-slate-500">{ep.desc}</span>
                </div>
              ))}
            </div>
          </div>
        </Card>
      </div>

      <div className="lg:col-span-1 space-y-6">
        <Card header={t('partners.api.auth_title')}>
          <div className="mb-4">
            <label className="block text-sm font-medium text-slate-700 mb-2 flex items-center gap-2">
              <Key className="w-4 h-4 text-slate-400" />
              {t('partners.api.key_label')}
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={apiKey}
                readOnly
                className="w-full bg-slate-50 border border-slate-300 rounded-lg text-sm px-3 py-2 text-slate-600 font-mono"
              />
              <Button size="sm" variant="secondary" onClick={handleCopyKey} leftIcon={copied ? <Shield className="w-4 h-4" /> : <Copy className="w-4 h-4" />}>
                {copied ? t('partners.common.copied') : t('partners.common.copy')}
              </Button>
            </div>
            <p className="text-xs text-slate-500 mt-2">
              {t('partners.api.key_warning')}
            </p>
          </div>

          <Button variant="outline" className="w-full" leftIcon={<Terminal className="w-4 h-4" />}>
            {t('partners.api.gen_key_btn')}
          </Button>
        </Card>

        <Card header={t('partners.api.limits_title')}>
          <div className="space-y-4">
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-slate-600">{t('partners.api.limit_rpm')}</span>
                <span className="font-medium text-slate-900">45 / 60</span>
              </div>
              <div className="w-full bg-slate-100 rounded-full h-2">
                <div className="bg-blue-500 h-2 rounded-full" style={{ width: '75%' }}></div>
              </div>
            </div>
            <div>
              <div className="flex justify-between text-sm mb-1">
                <span className="text-slate-600">{t('partners.api.limit_quota')}</span>
                <span className="font-medium text-slate-900">1,250 / 10,000</span>
              </div>
              <div className="w-full bg-slate-100 rounded-full h-2">
                <div className="bg-green-500 h-2 rounded-full" style={{ width: '12%' }}></div>
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
};

export default APIPortal;
