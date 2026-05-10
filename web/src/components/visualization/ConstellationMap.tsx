import React, { useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ScatterChart,
  Scatter,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ZAxis,
  ReferenceArea,
  ResponsiveContainer,
  Label,
} from 'recharts';
import { ConstellationData, ConstellationPoint } from '../../types/domain';
import Card from '../ui/Card';
import Button from '../ui/Button';
import { SkeletonCard } from '../ui/Skeleton';
import EmptyState from '../ui/EmptyState';
import { RefreshCw, MapPin } from 'lucide-react';

/* ───────────────────────────────────────────
   Constants
   ─────────────────────────────────────────── */

const OWN_COLOR = '#22C55E';
const COMPETITOR_COLOR = '#EF4444';
const PUBLIC_COLOR = '#94A3B8';
const WHITE_SPACE_COLOR = '#FDE68A';
const CLUSTER_LABEL_COLOR = '#475569';

const POINT_TYPES_CONFIG: Record<string, { color: string; label: string }> = {
  own_patent: { color: OWN_COLOR, label: 'Self-owned' },
  competitor_patent: { color: COMPETITOR_COLOR, label: 'Competitor' },
  public_patent: { color: PUBLIC_COLOR, label: 'Public' },
};

const POINT_TYPES = ['own_patent', 'competitor_patent', 'public_patent'];

interface TooltipPayloadEntry {
  name?: string;
  value?: number | string;
  color?: string;
  payload?: ConstellationPoint;
}

/* ───────────────────────────────────────────
   Custom Tooltip
   ─────────────────────────────────────────── */

interface CustomTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadEntry[];
}

const CustomTooltip: React.FC<CustomTooltipProps> = ({ active, payload }) => {
  if (!active || !payload || payload.length === 0) return null;

  const pt = payload[0]?.payload;
  if (!pt) return null;

  const cfg = POINT_TYPES_CONFIG[pt.point_type] || { color: PUBLIC_COLOR, label: pt.point_type };

  return (
    <div className="bg-white rounded-lg shadow-lg border border-slate-200 p-3 text-sm max-w-[220px]">
      <div className="flex items-center gap-2 mb-2">
        <span className="w-3 h-3 rounded-full" style={{ backgroundColor: cfg.color }} />
        <span className="font-semibold text-slate-900">{pt.patent_number}</span>
      </div>
      <div className="space-y-1 text-xs text-slate-600">
        <div className="flex justify-between gap-4">
          <span>Assignee:</span>
          <span className="font-medium text-slate-800 text-right">{pt.assignee || 'Unknown'}</span>
        </div>
        {pt.tech_domain && (
          <div className="flex justify-between gap-4">
            <span>Domain:</span>
            <span className="font-medium text-slate-800 text-right">{pt.tech_domain}</span>
          </div>
        )}
        <div className="flex justify-between gap-4">
          <span>Value Score:</span>
          <span className="font-medium text-slate-800 text-right">
            {pt.value_score != null ? pt.value_score.toFixed(1) : 'N/A'}
          </span>
        </div>
        {pt.filing_year && (
          <div className="flex justify-between gap-4">
            <span>Filing Year:</span>
            <span className="font-medium text-slate-800 text-right">{pt.filing_year}</span>
          </div>
        )}
        {pt.legal_status && (
          <div className="flex justify-between gap-4">
            <span>Status:</span>
            <span className="font-medium text-slate-800 text-right capitalize">{pt.legal_status}</span>
          </div>
        )}
        {pt.cluster_label && (
          <div className="flex justify-between gap-4">
            <span>Cluster:</span>
            <span className="font-medium text-slate-800 text-right">{pt.cluster_label}</span>
          </div>
        )}
      </div>
      <div className="mt-2 pt-2 border-t border-slate-100 text-center">
        <span className="text-blue-600 text-xs font-medium">Click to view patent details</span>
      </div>
    </div>
  );
};

/* ───────────────────────────────────────────
   Props
   ─────────────────────────────────────────── */

interface ConstellationMapProps {
  data: ConstellationData | null;
  loading: boolean;
  error: string | null;
  onRetry?: () => void;
}

/* ───────────────────────────────────────────
   Main Component
   ─────────────────────────────────────────── */

const ConstellationMap: React.FC<ConstellationMapProps> = ({ data, loading, error, onRetry }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();

  // Compute axis domain padding
  const { xMin, xMax, yMin, yMax } = useMemo(() => {
    if (!data || data.points.length === 0) {
      return { xMin: -5, xMax: 5, yMin: -5, yMax: 5 };
    }
    let xMin = Infinity, xMax = -Infinity;
    let yMin = Infinity, yMax = -Infinity;
    for (const pt of data.points) {
      if (pt.x < xMin) xMin = pt.x;
      if (pt.x > xMax) xMax = pt.x;
      if (pt.y < yMin) yMin = pt.y;
      if (pt.y > yMax) yMax = pt.y;
    }
    const xPad = (xMax - xMin) * 0.15 || 1;
    const yPad = (yMax - yMin) * 0.15 || 1;
    return { xMin: xMin - xPad, xMax: xMax + xPad, yMin: yMin - yPad, yMax: yMax + yPad };
  }, [data]);

  // Group points by type for separate Scatter traces
  const groupedPoints = useMemo(() => {
    if (!data) return {};
    const groups: Record<string, ConstellationPoint[]> = {};
    for (const pt of data.points) {
      const type = pt.point_type || 'public_patent';
      if (!groups[type]) groups[type] = [];
      groups[type].push(pt);
    }
    return groups;
  }, [data]);

  const handlePointClick = useCallback(
    (point: ConstellationPoint) => {
      if (point.patent_number) {
        navigate(`/patents/${point.id}`);
      }
    },
    [navigate],
  );

  // --- Loading state ---
  if (loading) {
    return (
      <Card header={<span className="font-semibold text-slate-800">Patent Constellation Map</span>}>
        <SkeletonCard rows={4} className="h-[500px]" />
      </Card>
    );
  }

  // --- Error state ---
  if (error) {
    return (
      <Card header={<span className="font-semibold text-slate-800">Patent Constellation Map</span>}>
        <div className="flex flex-col items-center justify-center py-12 gap-3">
          <div className="bg-red-50 p-3 rounded-full">
            <MapPin className="w-6 h-6 text-red-500" />
          </div>
          <p className="text-sm text-red-600">{error}</p>
          {onRetry && (
            <Button variant="outline" size="sm" onClick={onRetry} leftIcon={<RefreshCw className="w-4 h-4" />}>
              {t('common.retry', 'Retry')}
            </Button>
          )}
        </div>
      </Card>
    );
  }

  // --- Empty state ---
  if (!data || data.points.length === 0) {
    return (
      <Card header={<span className="font-semibold text-slate-800">Patent Constellation Map</span>}>
        <EmptyState
          icon={MapPin}
          title="No constellation data"
          description="No patent data is available for the constellation visualization."
          action={
            onRetry ? (
              <Button variant="outline" size="sm" onClick={onRetry} leftIcon={<RefreshCw className="w-4 h-4" />}>
                {t('common.retry', 'Retry')}
              </Button>
            ) : undefined
          }
        />
      </Card>
    );
  }

  const whiteSpaces = data.white_spaces || [];
  const clusters = data.clusters || [];

  // Build white space reference areas
  const wsReferenceAreas = whiteSpaces.map((ws) => (
    <ReferenceArea
      key={ws.region_id}
      x1={ws.center_x - 0.5}
      x2={ws.center_x + 0.5}
      y1={ws.center_y - 0.5}
      y2={ws.center_y + 0.5}
      fill={WHITE_SPACE_COLOR}
      fillOpacity={0.25}
      stroke="#F59E0B"
      strokeWidth={1}
      strokeDasharray="4 3"
    />
  ));

  return (
    <Card header={<span className="font-semibold text-slate-800">Patent Constellation Map</span>}>
      <div className="space-y-4">
        {/* Stats summary bar */}
        <div className="flex flex-wrap items-center gap-4 text-xs text-slate-600">
          <span className="font-medium text-slate-700">Summary:</span>
          <span>
            <span className="font-semibold text-slate-800">{data.total_points}</span> patents
          </span>
          <span>
            <span className="font-semibold text-green-600">{data.points.filter((p) => p.point_type === 'own_patent').length}</span> self-owned
          </span>
          <span>
            <span className="font-semibold text-red-500">{data.points.filter((p) => p.point_type === 'competitor_patent').length}</span> competitor
          </span>
          {clusters.length > 0 && (
            <span>
              <span className="font-semibold text-slate-800">{clusters.length}</span> clusters
            </span>
          )}
          {whiteSpaces.length > 0 && (
            <span>
              <span className="font-semibold text-amber-600">{whiteSpaces.length}</span> opportunities
            </span>
          )}
        </div>

        {/* Chart */}
        <div className="w-full h-[500px]">
          <ResponsiveContainer width="100%" height="100%">
            <ScatterChart
              margin={{ top: 30, right: 30, bottom: 30, left: 30 }}
            >
              <CartesianGrid strokeDasharray="3 3" stroke="#E2E8F0" />
              <XAxis
                type="number"
                dataKey="x"
                domain={[xMin, xMax]}
                tick={{ fontSize: 11, fill: '#94A3B8' }}
                tickLine={false}
                axisLine={{ stroke: '#E2E8F0' }}
              >
                <Label value="PC1 (t-SNE / PCA)" position="bottom" offset={-5} style={{ fontSize: 11, fill: '#94A3B8' }} />
              </XAxis>
              <YAxis
                type="number"
                dataKey="y"
                domain={[yMin, yMax]}
                tick={{ fontSize: 11, fill: '#94A3B8' }}
                tickLine={false}
                axisLine={{ stroke: '#E2E8F0' }}
              >
                <Label value="PC2 (t-SNE / PCA)" angle={-90} position="insideLeft" offset={10} style={{ fontSize: 11, fill: '#94A3B8' }} />
              </YAxis>
              <ZAxis type="number" dataKey="value_score" range={[40, 400]} />
              <Tooltip content={<CustomTooltip />} cursor={{ strokeDasharray: '3 3' }} />

              {/* Reference areas for white spaces */}
              {wsReferenceAreas}

              {/* Scatter traces for each point type */}
              {POINT_TYPES.map((ptType) => {
                const pts = groupedPoints[ptType] || [];
                if (pts.length === 0) return null;
                const cfg = POINT_TYPES_CONFIG[ptType] || { color: PUBLIC_COLOR, label: ptType };
                return (
                  <Scatter
                    key={ptType}
                    name={cfg.label}
                    data={pts}
                    fill={cfg.color}
                    stroke={cfg.color}
                    strokeWidth={0.5}
                    opacity={0.8}
                    shape="circle"
                    onClick={(pointData: unknown) => {
                      const entry = pointData as { payload?: ConstellationPoint };
                      if (entry?.payload) handlePointClick(entry.payload);
                    }}
                    className="cursor-pointer"
                  />
                );
              })}

              {/* Cluster labels rendered as custom SVG */}
              {clusters.length > 0 && (
                <g>
                  {clusters.map((cl) => {
                    const cx = ((cl.center_x - xMin) / (xMax - xMin)) * 100;
                    const cy = ((cl.center_y - yMax) / (yMin - yMax)) * 100;
                    return (
                      <g key={cl.cluster_id} style={{ pointerEvents: 'none' }}>
                        <rect
                          x={`${cx - 8}%`}
                          y={`${cy - 4}%`}
                          width="16%"
                          height="3%"
                          rx={6}
                          fill="white"
                          fillOpacity={0.9}
                          stroke="#CBD5E1"
                          strokeWidth={0.5}
                        />
                        <text
                          x={`${cx}%`}
                          y={`${cy - 1.5}%`}
                          textAnchor="middle"
                          fontSize={10}
                          fontWeight={700}
                          fill={CLUSTER_LABEL_COLOR}
                        >
                          {cl.label}
                        </text>
                        <text
                          x={`${cx}%`}
                          y={`${cy + 0.8}%`}
                          textAnchor="middle"
                          fontSize={8}
                          fill="#94A3B8"
                        >
                          {cl.point_count} patents
                        </text>
                      </g>
                    );
                  })}
                </g>
              )}

              {/* White space labels */}
              {whiteSpaces.length > 0 && (
                <g>
                  {whiteSpaces.map((ws) => {
                    const cx = ((ws.center_x - xMin) / (xMax - xMin)) * 100;
                    const cy = ((ws.center_y - yMax) / (yMin - yMax)) * 100;
                    return (
                      <g key={ws.region_id} style={{ pointerEvents: 'none' }}>
                        <circle
                          cx={`${cx}%`}
                          cy={`${cy}%`}
                          r={12}
                          fill={WHITE_SPACE_COLOR}
                          fillOpacity={0.25}
                          stroke="#F59E0B"
                          strokeWidth={1}
                          strokeDasharray="4 3"
                        />
                        <text
                          x={`${cx}%`}
                          y={`${cy}%`}
                          textAnchor="middle"
                          dominantBaseline="middle"
                          fontSize={8}
                          fontWeight={600}
                          fill="#B45309"
                        >
                          Opportunity
                        </text>
                      </g>
                    );
                  })}
                </g>
              )}

              <Legend
                verticalAlign="bottom"
                height={36}
                iconType="circle"
                formatter={(value: string) => (
                  <span className="text-xs text-slate-700">{value}</span>
                )}
              />
            </ScatterChart>
          </ResponsiveContainer>
        </div>

        {/* White space legend line */}
        {whiteSpaces.length > 0 && (
          <div className="flex items-center gap-2 text-xs text-amber-700 bg-amber-50 px-3 py-2 rounded-lg">
            <div className="w-4 h-4 rounded-full border-2 border-dashed border-amber-400 bg-amber-100" />
            <span>
              <strong>Opportunity areas</strong> highlight white-space gaps with low patent density.
              Click patents to navigate to details.
            </span>
          </div>
        )}
      </div>
    </Card>
  );
};

export default ConstellationMap;
