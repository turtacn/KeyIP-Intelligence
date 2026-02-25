import React, { useEffect, useRef, useState } from 'react';
import cytoscape from 'cytoscape';
import Card from '../../components/ui/Card';
import { Search, ZoomIn, ZoomOut, Maximize } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const KnowledgeGraph: React.FC = () => {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement>(null);
  const [cy, setCy] = useState<cytoscape.Core | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    if (!containerRef.current) return;

    const newCy = cytoscape({
      container: containerRef.current,
      elements: [
        // Mock Graph Data
        { data: { id: 'p1', label: 'Patent A', type: 'patent' } },
        { data: { id: 'p2', label: 'Patent B', type: 'patent' } },
        { data: { id: 'm1', label: 'Molecule X', type: 'molecule' } },
        { data: { id: 'c1', label: 'Company Y', type: 'company' } },
        { data: { source: 'p1', target: 'm1', label: 'CONTAINS' } },
        { data: { source: 'p2', target: 'm1', label: 'CONTAINS' } },
        { data: { source: 'p1', target: 'c1', label: 'OWNED_BY' } },
        { data: { source: 'p2', target: 'p1', label: 'CITES' } },
      ],
      style: [
        {
          selector: 'node',
          style: {
            'background-color': '#666',
            'label': 'data(label)',
            'color': '#fff',
            'text-valign': 'center',
            'text-halign': 'center',
            'font-size': '12px'
          }
        },
        {
          selector: 'node[type="patent"]',
          style: { 'background-color': '#3b82f6' } // Blue
        },
        {
          selector: 'node[type="molecule"]',
          style: { 'background-color': '#10b981', 'shape': 'diamond' } // Green
        },
        {
          selector: 'node[type="company"]',
          style: { 'background-color': '#f59e0b', 'shape': 'hexagon' } // Orange
        },
        {
          selector: 'edge',
          style: {
            'width': 2,
            'line-color': '#ccc',
            'target-arrow-color': '#ccc',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier'
          }
        },
        {
          selector: ':selected',
          style: {
            'border-width': 3,
            'border-color': '#000'
          }
        }
      ],
      layout: {
        name: 'cose',
        animate: false
      }
    });

    setCy(newCy);

    return () => {
      newCy.destroy();
    };
  }, []);

  const handleZoomIn = () => {
    cy?.zoom(cy.zoom() * 1.2);
  };

  const handleZoomOut = () => {
    cy?.zoom(cy.zoom() / 1.2);
  };

  const handleFit = () => {
    cy?.fit();
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!cy) return;
    cy.elements().removeClass('highlight'); // Reset logic needed if implementing highlight class
    const found = cy.nodes().filter((ele) => ele.data('label').toLowerCase().includes(searchTerm.toLowerCase()));
    if (found.length > 0) {
      cy.animate({
        fit: { eles: found, padding: 50 },
        duration: 500
      });
      found.select();
    }
  };

  return (
    <div className="h-[calc(100vh-8rem)] flex flex-col">
      <div className="mb-4 flex justify-between items-center">
        <h1 className="text-2xl font-bold text-slate-900">Knowledge Graph</h1>
        <div className="flex gap-2">
          <form onSubmit={handleSearch} className="relative">
            <input
              type="text"
              placeholder={t('knowledge_graph.search_placeholder')}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="pl-8 pr-4 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
            />
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          </form>
        </div>
      </div>

      <Card padding="none" className="flex-1 relative overflow-hidden border-slate-200 shadow-sm">
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

        {/* Legend Overlay */}
        <div className="absolute top-4 left-4 bg-white/90 p-3 rounded-lg shadow-sm border border-slate-200 text-xs pointer-events-none">
          <div className="font-semibold mb-2 text-slate-700">Legend</div>
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rounded-full bg-blue-500"></span> Patent
            </div>
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 rotate-45 bg-green-500"></span> Molecule
            </div>
            <div className="flex items-center gap-2">
              <span className="w-3 h-3 bg-amber-500" style={{ clipPath: 'polygon(50% 0%, 100% 25%, 100% 75%, 50% 100%, 0% 75%, 0% 25%)' }}></span> Company
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default KnowledgeGraph;
