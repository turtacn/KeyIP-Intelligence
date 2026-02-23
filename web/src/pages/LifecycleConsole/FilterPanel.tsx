import React from 'react';
import { Filter, Calendar, AlertCircle } from 'lucide-react';
import { Jurisdiction } from '../../types/domain';

interface FilterPanelProps {
  filters: {
    jurisdiction: Jurisdiction | 'All';
    eventType: string;
    riskLevel: string;
    startDate: string;
    endDate: string;
  };
  onFilterChange: (newFilters: any) => void;
  onReset: () => void;
}

const FilterPanel: React.FC<FilterPanelProps> = ({ filters, onFilterChange, onReset }) => {
  const jurisdictions: (Jurisdiction | 'All')[] = ['All', 'CN', 'US', 'EP', 'JP', 'KR', 'Other'];
  const eventTypes = ['All', 'annuity_due', 'response_deadline', 'examination', 'grant_expected', 'expiry_warning'];
  const riskLevels = ['All', 'Overdue', 'Due 7d', 'Due 30d', 'Due 90d'];

  const handleChange = (key: string, value: string) => {
    onFilterChange({ ...filters, [key]: value });
  };

  return (
    <div className="bg-white rounded-lg border border-slate-200 p-5 space-y-6 sticky top-24">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold text-slate-900 flex items-center">
          <Filter className="w-4 h-4 mr-2" />
          Filters
        </h3>
        <button
          onClick={onReset}
          className="text-xs text-blue-600 hover:text-blue-800 font-medium"
        >
          Reset
        </button>
      </div>

      <div className="space-y-4">
        {/* Jurisdiction Filter */}
        <div>
          <label className="block text-xs font-medium text-slate-500 mb-2">Jurisdiction</label>
          <div className="flex flex-wrap gap-2">
            {jurisdictions.map((j) => (
              <button
                key={j}
                onClick={() => handleChange('jurisdiction', j)}
                className={`px-2 py-1 text-xs rounded-md border transition-colors ${
                  filters.jurisdiction === j
                    ? 'bg-blue-50 border-blue-200 text-blue-700 font-medium'
                    : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50'
                }`}
              >
                {j}
              </button>
            ))}
          </div>
        </div>

        {/* Event Type Filter */}
        <div>
          <label className="block text-xs font-medium text-slate-500 mb-2">Event Type</label>
          <select
            value={filters.eventType}
            onChange={(e) => handleChange('eventType', e.target.value)}
            className="w-full text-sm border-slate-300 rounded-md shadow-sm focus:border-blue-500 focus:ring-blue-500"
          >
            {eventTypes.map((type) => (
              <option key={type} value={type}>
                {type === 'All' ? 'All Event Types' : type.replace(/_/g, ' ')}
              </option>
            ))}
          </select>
        </div>

        {/* Date Range Filter */}
        <div>
          <label className="block text-xs font-medium text-slate-500 mb-2">
            <div className="flex items-center">
              <Calendar className="w-3 h-3 mr-1" />
              Due Date Range
            </div>
          </label>
          <div className="grid grid-cols-2 gap-2">
            <input
              type="date"
              value={filters.startDate}
              onChange={(e) => handleChange('startDate', e.target.value)}
              className="text-xs border-slate-300 rounded-md focus:ring-blue-500 focus:border-blue-500 w-full"
            />
            <input
              type="date"
              value={filters.endDate}
              onChange={(e) => handleChange('endDate', e.target.value)}
              className="text-xs border-slate-300 rounded-md focus:ring-blue-500 focus:border-blue-500 w-full"
            />
          </div>
        </div>

        {/* Urgency/Risk Filter */}
        <div>
          <label className="block text-xs font-medium text-slate-500 mb-2">
            <div className="flex items-center">
              <AlertCircle className="w-3 h-3 mr-1" />
              Urgency
            </div>
          </label>
          <div className="space-y-2">
            {riskLevels.map((level) => (
              <label key={level} className="flex items-center">
                <input
                  type="radio"
                  name="riskLevel"
                  value={level}
                  checked={filters.riskLevel === level}
                  onChange={(e) => handleChange('riskLevel', e.target.value)}
                  className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-slate-300"
                />
                <span className="ml-2 text-sm text-slate-600">{level}</span>
              </label>
            ))}
          </div>
        </div>
      </div>

      <div className="pt-4 border-t border-slate-100">
        <button
          className="w-full bg-blue-600 text-white py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors shadow-sm"
          onClick={() => {}} // Could trigger a specific "Apply" action if not instant
        >
          Apply Filters
        </button>
      </div>
    </div>
  );
};

export default FilterPanel;
