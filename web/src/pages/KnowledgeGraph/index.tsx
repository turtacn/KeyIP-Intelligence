import React, { useEffect, useRef, useState } from 'react';
import cytoscape from 'cytoscape';
import Card from '../../components/ui/Card';
import Badge from '../../components/ui/Badge';
import Button from '../../components/ui/Button';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import { Search, ZoomIn, ZoomOut, Maximize, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useCitationNetwork } from '../../hooks/useKnowledgeGraph';
import type { CitationRef } from '../../services/knowledgeGraph.service';

interface GraphNode {
  data: {
    id: string;
    label: string;
    type: 'patent' | 'citation';
    relation?: 'cites' | 'cited_by';
  };
}

interface GraphEdge {
  data: {
    source: string;
    target: string;
    label: string;
    directed?: boolean;
  };
}

interface GraphElements {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

const KnowledgeGraph: React.FC = () => {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<cytoscape.Core | null>(null);
  const [searchTerm, setSearchTerm] = useState('');
  const [patentId, setPatentId] = useState('');
  const [inputPatentId, setInputPatentId] = useState('');
  const [mode, setMode] = useState<'mock' | 'realtime'>('mock');

  const { data: citationData, loading: citationLoading, error: citationError } = useCitationNetwork(
    mode === 'realtime' && patentId ? patentId : undefined
  );

  const generateMockElements = (): GraphElements => ({
    nodes: [
      { data: { id: 'p1', label: 'Patent A', type: 'patent' } },
      { data: { id: 'p2', label: 'Patent B', type: 'patent' } },
      { data: { id: 'm1', label: 'Molecule X', type: 'citation' } },
      { data: { id: 'c1', label: 'Company Y', type: 'citation' } },
    ],
    edges: [
      { data: { source: 'p1', target: 'm1', label: 'CONTAINS' } },
      { data: { source: 'p2', target: 'm1', label: 'CONTAINS' } },
      { data: { source: 'p1', target: 'c1', label: 'OWNED_BY' } },
      { data: { source: 'p2', target: 'p1', label: 'CITES' } },
    ],
  });

  const buildElementsFromCitationData = (): GraphElements => {
    if (!citationData) return { nodes: [], edges: [] };

    const nodes: GraphNode[] = [
      { data: { id: 'center', label: citationData.patent_number, type: 'patent' } },
    ];
    const edges: GraphEdge[] = [];
    const seen = new Set<string>(['center']);

    citationData.backward_citations.forEach((ref: CitationRef, idx: number) => {
      const nodeId = `back-${idx}`;
      if (!seen.has(nodeId)) {
        nodes.push({ data: { id: nodeId, label: ref.patent_number, type: 'citation', relation: 'cited_by' } });
        seen.add(nodeId);
      }
      edges.push({ data: { source: nodeId, target: 'center', label: 'CITED_BY' } });
    });

    citationData.forward_citations.forEach((ref: CitationRef, idx: number) => {
      const nodeId = `fwd-${idx}`;
      if (!seen.has(nodeId)) {
        nodes.push({ data: { id: nodeId, label: ref.patent_number, type: 'citation', relation: 'cites' } });
        seen.add(nodeId);
      }
      edges.push({ data: { source: 'center', target: nodeId, label: 'CITES' } });
    });

    return { nodes, edges };
  };

  // Initialize or update cytoscape
  useEffect(() => {
    if (!containerRef.current) return;
    const container = containerRef.current;

    // Destroy previous instance
    if (cyRef.current) {
      cyRef.current.destroy();
      cyRef.current = null;
    }

    const elements = mode === 'realtime' && citationData
      ? buildElementsFromCitationData()
      : generateMockElements();

    const newCy = cytoscape({
      container,
      elements: [...elements.nodes, ...elements.edges],
      style: [
        {
          selector: 'node',
          style: {
            'background-color': '#666',
            label: 'data(label)',
            color: '#fff',
            'text-valign': 'center',
            'text-halign': 'center',
            'font-size': '10px',
            width: '40px',
            height: '40px',
          },
        },
        {
          selector: 'node[type="patent"]',
          style: { 'background-color': '#3b82f6', width: '50px', height: '50px', 'font-size': '11px' },
        },
        {
          selector: 'node[type="citation"]',
          style: { 'background-color': '#10b981', shape: 'diamond' },
        },
        {
          selector: 'node:selected',
          style: { 'border-width': 3, 'border-color': '#f59e0b', 'border-opacity': 1 },
        },
        {
          selector: 'edge',
          style: {
            width: 1.5,
            'line-color': '#94a3b8',
            'target-arrow-color': '#94a3b8',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            'font-size': '8px',
            label: 'data(label)',
            color: '#64748b',
            'text-background-color': '#ffffff',
            'text-background-opacity': 0.8,
            'text-background-padding': '2px',
          },
        },
        {
          selector: 'edge[directed="false"]',
          style: { 'target-arrow-shape': 'none' },
        },
      ],
      layout: {
        name: 'cose',
        animate: false,
        padding: 30,
      },
      wheelSensitivity: 0.3,
    });

    // Add tap handler for nodes
    newCy.on('tap', 'node', (evt) => {
      const node = evt.target;
      const nodeId = node.data('id');
      if (nodeId && nodeId !== 'center') {
        const relatedEdges = newCy.edges().filter(e => e.data('source') === nodeId || e.data('target') === nodeId);
        newCy.animate({
          fit: { eles: node.union(relatedEdges), padding: 50 },
          duration: 300,
        });
      }
    });

    cyRef.current = newCy;

    return () => {
      newCy.destroy();
      cyRef.current = null;
    };
  }, [citationData, mode]);

  const handleZoomIn = () => cyRef.current?.zoom(cyRef.current.zoom() * 1.3);
  const handleZoomOut = () => cyRef.current?.zoom(cyRef.current.zoom() / 1.3);
  const handleFit = () => cyRef.current?.fit(undefined, 50);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const cy = cyRef.current;
    if (!cy) return;

    const term = searchTerm.toLowerCase();
    if (!term) {
      cy.elements().unselect();
      cy.fit(undefined, 50);
      return;
    }

    const found = cy.nodes().filter((ele) =>
      ele.data('label')?.toLowerCase().includes(term)
    );
    if (found.length > 0) {
      cy.animate({
        fit: { eles: found, padding: 50 },
        duration: 500,
      });
      found.select();
    }
  };

  const handleLoadPatentNetwork = (e: React.FormEvent) => {
    e.preventDefault();
    const pid = inputPatentId.trim();
    if (!pid) return;
    setMode('realtime');
    setPatentId(pid);
  };

  const handleResetMock = () => {
    setMode('mock');
    setPatentId('');
    setInputPatentId('');
    if (cyRef.current) {
      cyRef.current.destroy();
      cyRef.current = null;
    }
  };

  const isRealtimeLoaded = mode === 'realtime' && citationData;

  return (
    <div className="h-[calc(100vh-8rem)] flex flex-col">
      <div className="mb-4 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
        <h1 className="text-2xl font-bold text-slate-900">Knowledge Graph</h1>
        <div className="flex flex-wrap items-center gap-2">
          {/* Search within graph */}
          <form onSubmit={handleSearch} className="relative">
            <input
              type="text"
              placeholder={t('knowledge_graph.search_placeholder')}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-8 pr-3 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500 w-40"
            />
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          </form>

          {/* Load citation network */}
          <form onSubmit={handleLoadPatentNetwork} className="flex items-center gap-1">
            <input
              type="text"
              placeholder="Patent ID for network"
              value={inputPatentId}
              onChange={(e) => setInputPatentId(e.target.value)}
              className="pl-3 pr-3 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500 w-40"
            />
            <Button type="submit" size="sm" variant="outline" isLoading={citationLoading}>
              Load
            </Button>
          </form>

          {mode === 'realtime' && (
            <Button size="sm" variant="ghost" onClick={handleResetMock} leftIcon={<RotateCcw className="w-4 h-4" />}>
              Reset
            </Button>
          )}
        </div>
      </div>

      <Card padding="none" className="flex-1 relative overflow-hidden border-slate-200 shadow-sm">
        {/* Loading overlay for real-time mode */}
        {mode === 'realtime' && citationLoading && (
          <div className="absolute inset-0 bg-white/70 z-10 flex items-center justify-center">
            <div className="text-center">
              <LoadingSpinner size="lg" />
              <p className="mt-2 text-sm text-slate-500">Loading citation network...</p>
            </div>
          </div>
        )}

        {/* Error overlay for real-time mode */}
        {mode === 'realtime' && citationError && (
          <div className="absolute top-4 left-1/2 -translate-x-1/2 z-10 bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded-lg shadow-md text-sm">
            Failed to load citation network: {citationError}
          </div>
        )}

        {/* Cytoscape container */}
        <div ref={containerRef} className="absolute inset-0 bg-slate-50" />

        {/* Graph Controls */}
        <div className="absolute bottom-4 right-4 flex flex-col gap-2 bg-white p-2 rounded-lg shadow-md border border-slate-200">
          <button onClick={handleZoomIn} className="p-2 hover:bg-slate-100 rounded text-slate-600" title="Zoom In">
            <ZoomIn className="w-5 h-5" />
          </button>
          <button onClick={handleZoomOut} className="p-2 hover:bg-slate-100 rounded text-slate-600" title="Zoom Out">
            <ZoomOut className="w-5 h-5" />
          </button>
          <button onClick={handleFit} className="p-2 hover:bg-slate-100 rounded text-slate-600" title="Fit to Screen">
            <Maximize className="w-5 h-5" />
          </button>
        </div>

        {/* Legend */}
        <div className="absolute top-4 left-4 bg-white/90 p-3 rounded-lg shadow-sm border border-slate-200 text-xs">
          <div className="font-semibold mb-2 text-slate-700">Legend</div>
          <div className="space-y-1.5">
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-full bg-blue-500"></span>
              <span>Patent</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 bg-green-500" style={{ clipPath: 'polygon(50% 0%, 100% 50%, 50% 100%, 0% 50%)' }}></span>
              <span>Citation</span>
            </div>
            <div className="flex items-center gap-2 mt-2 text-slate-400">
              <span className="text-[10px]">Click a node to focus</span>
            </div>
          </div>
        </div>

        {/* Mode indicator */}
        <div className="absolute top-4 right-4">
          <Badge variant={mode === 'realtime' ? 'info' : 'default'} size="sm">
            {mode === 'realtime' ? 'Citation Network' : 'Demo View'}
          </Badge>
        </div>
      </Card>

      {/* Stats bar for realtime mode */}
      {isRealtimeLoaded && (
        <div className="mt-2 flex items-center gap-4 text-xs text-slate-500">
          <span>Patent: <strong>{citationData.patent_number}</strong></span>
          <span>Total Citations: <strong>{citationData.total_citations}</strong></span>
          <span>Forward: <strong>{citationData.forward_citations.length}</strong></span>
          <span>Backward: <strong>{citationData.backward_citations.length}</strong></span>
        </div>
      )}
    </div>
  );
};

export default KnowledgeGraph;
