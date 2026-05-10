import React, { useEffect, useMemo, useRef, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { FamilyResponse, FamilyMember } from '../../types/domain';
import Card from '../ui/Card';
import { SkeletonCard } from '../ui/Skeleton';
import Button from '../ui/Button';
import { RefreshCw, Users } from 'lucide-react';

/* ───────────────────────────────────────────
   Types, constants & helpers
   ─────────────────────────────────────────── */

const JURISDICTION_COLORS: Record<string, { bg: string; border: string; text: string; label: string }> = {
  CN: { bg: '#FEF2F2', border: '#DC2626', text: '#991B1B', label: 'CN' },
  US: { bg: '#EFF6FF', border: '#2563EB', text: '#1E3A5F', label: 'US' },
  EP: { bg: '#F0FDF4', border: '#16A34A', text: '#14532D', label: 'EP' },
  JP: { bg: '#FFF7ED', border: '#EA580C', text: '#9A3412', label: 'JP' },
  KR: { bg: '#FAF5FF', border: '#7C3AED', text: '#4C1D95', label: 'KR' },
  WO: { bg: '#F8FAFC', border: '#64748B', text: '#334155', label: 'WO' },
};

const DEFAULT_JURIS_COLOR = { bg: '#F8FAFC', border: '#94A3B8', text: '#475569', label: 'OT' };

const STATUS_COLORS: Record<string, { fill: string; label: string }> = {
  granted: { fill: '#16A34A', label: 'Granted' },
  pending: { fill: '#EAB308', label: 'Pending' },
  expired: { fill: '#DC2626', label: 'Expired' },
  abandoned: { fill: '#6B7280', label: 'Abandoned' },
};

function getJurisdictionStyle(jurisdiction: string) {
  const key = jurisdiction.toUpperCase();
  return JURISDICTION_COLORS[key] ?? DEFAULT_JURIS_COLOR;
}

function getStatusStyle(status?: string) {
  if (!status) return null;
  const key = status.toLowerCase();
  return STATUS_COLORS[key] ?? null;
}

function truncate(text: string, max: number): string {
  if (text.length <= max) return text;
  return text.slice(0, max - 1) + '…';
}

/* ───────────────────────────────────────────
   Tree layout data structures
   ─────────────────────────────────────────── */

interface TreeNode {
  id: string;
  patentNumber: string;
  title: string;
  jurisdiction: string;
  relationship: string;
  legalStatus?: string;
  isRoot: boolean;
  x: number;
  y: number;
}

/* ───────────────────────────────────────────
   Layout constants
   ─────────────────────────────────────────── */

const NODE_W = 200;
const NODE_H = 82;
const H_GAP = 24;
const V_GAP = 56;
const MARGIN = { top: 24, right: 32, bottom: 24, left: 32 };
const ROOT_Y = MARGIN.top + NODE_H / 2;
const CHILD_Y = ROOT_Y + NODE_H / 2 + V_GAP + NODE_H / 2;

/* ───────────────────────────────────────────
   Sub-components
   ─────────────────────────────────────────── */

interface NodeProps {
  node: TreeNode;
  jurisdictionStyle: ReturnType<typeof getJurisdictionStyle>;
  statusStyle: ReturnType<typeof getStatusStyle>;
  onClick: (id: string) => void;
}

const TreeNodeShape: React.FC<NodeProps> = ({ node, jurisdictionStyle, statusStyle, onClick }) => {
  const cx = node.x;
  const cy = node.y;
  const hw = NODE_W / 2;
  const hh = NODE_H / 2;

  return (
    <g
      style={{ cursor: 'pointer' }}
      onClick={() => onClick(node.id)}
      role="link"
      aria-label={`${node.patentNumber}: ${node.title}`}
    >
      {/* Shadow */}
      <defs>
        <filter id={`shadow-${node.id}`} x="-10%" y="-10%" width="130%" height="130%">
          <feDropShadow dx={0} dy={2} stdDeviation={3} floodOpacity={0.12} />
        </filter>
      </defs>

      {/* Background */}
      <rect
        x={cx - hw}
        y={cy - hh}
        width={NODE_W}
        height={NODE_H}
        rx={8}
        ry={8}
        fill={jurisdictionStyle.bg}
        stroke={jurisdictionStyle.border}
        strokeWidth={node.isRoot ? 2.5 : 1.5}
        filter={`url(#shadow-${node.id})`}
      />

      {/* Left jurisdiction accent bar */}
      <rect
        x={cx - hw}
        y={cy - hh}
        width={6}
        height={NODE_H}
        rx={8}
        ry={8}
        fill={jurisdictionStyle.border}
        clipPath={`inset(0 ${NODE_W - 6}px 0 0 round 8px)`}
        style={{ pointerEvents: 'none' }}
      />
      {/* Manually draw a left accent using a path to avoid clip-path issues */}
      <path
        d={`M${cx - hw},${cy - hh + 8} Q${cx - hw},${cy - hh} ${cx - hw + 4},${cy - hh} L${cx - hw + 6},${cy - hh} L${cx - hw + 6},${cy + hh} L${cx - hw + 4},${cy + hh} Q${cx - hw},${cy + hh} ${cx - hw},${cy + hh - 8} Z`}
        fill={jurisdictionStyle.border}
      />

      {/* Jurisdiction badge */}
      <rect
        x={cx - hw + 14}
        y={cy - hh + 10}
        width={28}
        height={18}
        rx={4}
        fill={jurisdictionStyle.border}
        opacity={0.15}
      />
      <text
        x={cx - hw + 28}
        y={cy - hh + 23}
        textAnchor="middle"
        fontSize={10}
        fontWeight={700}
        fill={jurisdictionStyle.text}
        style={{ pointerEvents: 'none' }}
      >
        {jurisdictionStyle.label}
      </text>

      {/* Patent number */}
      <text
        x={cx - hw + 50}
        y={cy - hh + 23}
        fontSize={12}
        fontWeight={600}
        fill={jurisdictionStyle.text}
        style={{ pointerEvents: 'none' }}
      >
        {truncate(node.patentNumber, 22)}
      </text>

      {/* Title */}
      <text
        x={cx - hw + 14}
        y={cy - hh + 46}
        fontSize={11}
        fill="#64748B"
        style={{ pointerEvents: 'none' }}
      >
        {truncate(node.title, 30)}
      </text>

      {/* Relationship label for non-root nodes */}
      {!node.isRoot && (
        <text
          x={cx - hw + 14}
          y={cy - hh + 62}
          fontSize={10}
          fill="#94A3B8"
          fontStyle="italic"
          style={{ pointerEvents: 'none' }}
        >
          {node.relationship.replace(/_/g, ' ')}
        </text>
      )}

      {/* Status indicator */}
      {statusStyle && (
        <>
          <circle
            cx={cx + hw - 16}
            cy={cy - hh + 13}
            r={5}
            fill={statusStyle.fill}
            stroke="#fff"
            strokeWidth={1.5}
            style={{ pointerEvents: 'none' }}
          />
          <text
            x={cx + hw - 22}
            y={cy - hh + 25}
            textAnchor="end"
            fontSize={9}
            fill={statusStyle.fill}
            fontWeight={500}
            style={{ pointerEvents: 'none' }}
          >
            {statusStyle.label}
          </text>
        </>
      )}

      {/* Root indicator */}
      {node.isRoot && (
        <text
          x={cx}
          y={cy + hh - 5}
          textAnchor="middle"
          fontSize={9}
          fill="#94A3B8"
          fontStyle="italic"
          style={{ pointerEvents: 'none' }}
        >
          Current patent
        </text>
      )}
    </g>
  );
};

/* ───────────────────────────────────────────
   Main component
   ─────────────────────────────────────────── */

interface FamilyTreeProps {
  familyData: FamilyResponse | null;
  loading: boolean;
  error: string | null;
  onRetry?: () => void;
}

const FamilyTree: React.FC<FamilyTreeProps> = ({ familyData, loading, error, onRetry }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  // Track container width for responsive SVG
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });
    observer.observe(el);
    setContainerWidth(el.clientWidth);
    return () => observer.disconnect();
  }, []);

  // Build tree nodes from API response
  const treeNodes: TreeNode[] = useMemo(() => {
    if (!familyData) return [];

    const root: TreeNode = {
      id: familyData.patent_id,
      patentNumber: familyData.patent_number,
      title: 'Current Patent',
      jurisdiction: '',
      relationship: '',
      isRoot: true,
      x: 0,
      y: 0,
    };

    const children: TreeNode[] = familyData.members.map((m: FamilyMember) => ({
      id: m.id,
      patentNumber: m.patent_number,
      title: m.title,
      jurisdiction: m.jurisdiction,
      relationship: m.relationship,
      legalStatus: m.legal_status,
      isRoot: false,
      x: 0,
      y: 0,
    }));

    return [root, ...children];
  }, [familyData]);

  // Compute layout positions
  const layout = useMemo(() => {
    if (treeNodes.length === 0) return { nodes: [], svgWidth: 600, svgHeight: 200 };

    const root = treeNodes[0];
    const children = treeNodes.slice(1);

    const totalChildWidth = children.length * NODE_W + (children.length - 1) * H_GAP;
    const minSvgWidth = NODE_W + MARGIN.left + MARGIN.right;
    const calculatedWidth = Math.max(minSvgWidth, totalChildWidth + MARGIN.left + MARGIN.right);

    // Use available container width if it's larger
    const svgWidth = Math.max(calculatedWidth, containerWidth || calculatedWidth);
    const svgHeight = children.length > 0
      ? MARGIN.top + NODE_H + V_GAP + NODE_H + MARGIN.bottom + 8
      : MARGIN.top + NODE_H + MARGIN.bottom;

    // Layout root at top center
    root.x = svgWidth / 2;
    root.y = ROOT_Y;

    // Layout children evenly spaced below root
    const startX = (svgWidth - totalChildWidth) / 2 + NODE_W / 2;
    children.forEach((child, i) => {
      child.x = startX + i * (NODE_W + H_GAP);
      child.y = CHILD_Y;
    });

    return { nodes: treeNodes, svgWidth, svgHeight };
  }, [treeNodes, containerWidth]);

  // Paths from root to each child
  const paths = useMemo(() => {
    if (layout.nodes.length < 2) return [];
    const root = layout.nodes[0];
    return layout.nodes.slice(1).map((child) => {
      const midY = (root.y + child.y) / 2;
      return `M ${root.x},${root.y + NODE_H / 2} C ${root.x},${midY} ${child.x},${midY} ${child.x},${child.y - NODE_H / 2}`;
    });
  }, [layout]);

  const handleNodeClick = useCallback(
    (id: string) => {
      navigate(`/patents/${id}`);
    },
    [navigate],
  );

  // --- Loading state ---
  if (loading) {
    return (
      <Card header={<span className="font-semibold text-slate-800">Patent Family Tree</span>}>
        <SkeletonCard rows={1} className="h-48" />
      </Card>
    );
  }

  // --- Error state ---
  if (error) {
    return (
      <Card header={<span className="font-semibold text-slate-800">Patent Family Tree</span>}>
        <div className="flex flex-col items-center justify-center py-8 gap-3">
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
  if (!familyData || treeNodes.length === 0) {
    return (
      <Card header={<span className="font-semibold text-slate-800">Patent Family Tree</span>}>
        <div className="flex flex-col items-center justify-center py-8 gap-2">
          <Users className="w-8 h-8 text-slate-300" />
          <p className="text-sm text-slate-400">No family members found for this patent.</p>
        </div>
      </Card>
    );
  }

  const hasMultipleChildren = layout.nodes.length > 1;
  const svgViewBox = hasMultipleChildren
    ? `${-MARGIN.left} 0 ${layout.svgWidth + MARGIN.left + MARGIN.right} ${layout.svgHeight + 4}`
    : '0 0 600 140';

  return (
    <Card header={<span className="font-semibold text-slate-800">Patent Family Tree</span>}>
      <div className="space-y-4">
        {/* Legend */}
        <div className="flex flex-wrap items-center gap-x-6 gap-y-1 text-xs">
          <span className="font-medium text-slate-500 mr-1">Jurisdictions:</span>
          {Object.entries(JURISDICTION_COLORS).map(([key, val]) => (
            <span key={key} className="inline-flex items-center gap-1.5">
              <span
                className="inline-block w-2.5 h-2.5 rounded-full"
                style={{ backgroundColor: val.border }}
              />
              {key}
            </span>
          ))}
          <span className="font-medium text-slate-500 ml-2 mr-1">Status:</span>
          {Object.values(STATUS_COLORS).map((val) => (
            <span key={val.label} className="inline-flex items-center gap-1.5">
              <span
                className="inline-block w-2.5 h-2.5 rounded-full"
                style={{ backgroundColor: val.fill }}
              />
              {val.label}
            </span>
          ))}
        </div>

        {/* SVG tree */}
        <div ref={containerRef} className="w-full overflow-auto">
          <svg
            viewBox={svgViewBox}
            width="100%"
            style={{ maxHeight: layout.svgHeight + 24, minHeight: 140 }}
            className="select-none"
            role="img"
            aria-label="Patent family tree visualization"
          >
            {/* Connecting paths */}
            {paths.map((d, i) => (
              <path
                key={i}
                d={d}
                fill="none"
                stroke="#CBD5E1"
                strokeWidth={2}
                strokeLinecap="round"
              />
            ))}

            {/* Nodes */}
            {layout.nodes.map((node) => {
              const jurisStyle = node.isRoot
                ? { bg: '#F1F5F9', border: '#64748B', text: '#1E293B', label: '' }
                : getJurisdictionStyle(node.jurisdiction);
              const statusStyle = node.isRoot ? null : getStatusStyle(node.legalStatus);

              return (
                <TreeNodeShape
                  key={node.id}
                  node={node}
                  jurisdictionStyle={jurisStyle}
                  statusStyle={statusStyle}
                  onClick={handleNodeClick}
                />
              );
            })}

            {/* No children hint */}
            {!hasMultipleChildren && (
              <text
                x={300}
                y={120}
                textAnchor="middle"
                fontSize={13}
                fill="#94A3B8"
              >
                No related family members
              </text>
            )}
          </svg>
        </div>
      </div>
    </Card>
  );
};

export default FamilyTree;
