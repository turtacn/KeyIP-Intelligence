import React, { useEffect, useRef, useState, useCallback } from 'react';
import {
  getRDKit,
  RDKitModule,
  RDKitMolecule,
  RDKitMoleculeJSON,
  CPK_COLORS,
} from '../../utils/rdkitLoader';
import LoadingSpinner from './LoadingSpinner';

// ---------------------------------------------------------------------------
// Public props
// ---------------------------------------------------------------------------

export interface MoleculeViewerProps {
  /** SMILES string of the molecule to render */
  smiles: string;
  /** Viewer width in pixels (default 300) */
  width?: number;
  /** Viewer height in pixels (default 200) */
  height?: number;
  /** Additional CSS class names */
  className?: string;
  /** SMARTS pattern to highlight as substructure */
  substructure?: string;
  /** Hex colour for substructure highlights (default #FF4444) */
  highlightColor?: string;
  /** Apply CPK element colouring when true */
  atomColoring?: boolean;
  /** Show the zoom/pan/export toolbar (default true) */
  showControls?: boolean;
  /** Called when an atom label is clicked */
  onAtomClick?: (index: number, symbol: string) => void;
  /** Called when a bond is clicked */
  onBondClick?: (index: number, beginIdx: number, endIdx: number) => void;
}

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

interface ViewTransform {
  x: number;
  y: number;
  scale: number;
}

interface TooltipState {
  type: 'atom' | 'bond';
  text: string;
  x: number;
  y: number;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Parse a hex colour string (#RRGGBB) to an RDKit RGB tuple [0-1, 0-1, 0-1]. */
const hexToRgb = (hex: string): [number, number, number] | null => {
  const m = /^#?([0-9a-fA-F]{2})([0-9a-fA-F]{2})([0-9a-fA-F]{2})$/.exec(hex);
  return m
    ? [parseInt(m[1], 16) / 255, parseInt(m[2], 16) / 255, parseInt(m[3], 16) / 255]
    : null;
};

/** Trigger a file download from a Blob. */
const downloadBlob = (blob: Blob, filename: string) => {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
};

const BOND_TYPE_LABELS: Record<number, string> = {
  1: 'Single',
  2: 'Double',
  3: 'Triple',
  4: 'Aromatic',
  5: 'Sextuple',
};

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const MoleculeViewer: React.FC<MoleculeViewerProps> = ({
  smiles,
  width = 300,
  height = 200,
  className = '',
  substructure,
  highlightColor = '#FF4444',
  atomColoring = false,
  showControls = true,
  onAtomClick,
  onBondClick,
}) => {
  // --- refs ---
  const containerRef = useRef<HTMLDivElement>(null);
  const svgWrapperRef = useRef<HTMLDivElement>(null);
  const molRef = useRef<RDKitMolecule | null>(null);
  const rdkitRef = useRef<RDKitModule | null>(null);
  const dragState = useRef<{
    active: boolean;
    startX: number;
    startY: number;
    startTransform: ViewTransform;
  }>({ active: false, startX: 0, startY: 0, startTransform: { x: 0, y: 0, scale: 1 } });

  // --- state ---
  const [svgContent, setSvgContent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [molData, setMolData] = useState<RDKitMoleculeJSON | null>(null);
  const [transform, setTransform] = useState<ViewTransform>({ x: 0, y: 0, scale: 1 });
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const [hasSubstructMatch, setHasSubstructMatch] = useState(false);

  // Stable callbacks --------------------------------------------------------

  const resetView = useCallback(() => {
    setTransform({ x: 0, y: 0, scale: 1 });
  }, []);

  const zoomIn = useCallback(() => {
    setTransform((prev) => ({
      ...prev,
      scale: Math.min(10, prev.scale * 1.25),
    }));
  }, []);

  const zoomOut = useCallback(() => {
    setTransform((prev) => ({
      ...prev,
      scale: Math.max(0.1, prev.scale * 0.8),
    }));
  }, []);

  // --- Main effect: load molecule & generate SVG ---
  useEffect(() => {
    let mounted = true;
    const currentMol = molRef.current;
    if (currentMol) {
      currentMol.delete();
      molRef.current = null;
    }
    rdkitRef.current = null;
    setMolData(null);
    setTooltip(null);
    setHasSubstructMatch(false);

    if (!smiles) {
      setLoading(false);
      setSvgContent('');
      return;
    }

    const render = async () => {
      try {
        setLoading(true);
        setError(null);

        const rdkit: RDKitModule = await getRDKit();
        if (!mounted) return;
        rdkitRef.current = rdkit;

        const mol = rdkit.get_mol(smiles);
        if (!mounted) return;
        if (!mol) {
          setError('Invalid SMILES');
          setLoading(false);
          return;
        }
        molRef.current = mol;

        // Parse JSON data (atoms & bonds) for tooltips / atom-colouring
        let parsed: RDKitMoleculeJSON | null = null;
        try {
          const raw = mol.get_json();
          parsed = JSON.parse(raw);
        } catch {
          // non-fatal — tooltip/colouring will be unavailable
        }
        if (!mounted) return;
        setMolData(parsed);

        // Build drawing options
        const options: Record<string, unknown> = {};
        let hasOptions = false;

        // -- atom colouring --
        if (atomColoring && parsed) {
          const atomColours: Record<number, [number, number, number]> = {};
          for (let i = 0; i < parsed.atoms.length; i++) {
            const cpk = CPK_COLORS[parsed.atoms[i].symbol];
            if (cpk) atomColours[i] = cpk;
          }
          if (Object.keys(atomColours).length > 0) {
            options.atomColours = atomColours;
            hasOptions = true;
          }
        }

        // -- substructure highlighting --
        if (substructure) {
          const qmol = rdkit.get_qmol(substructure);
          if (qmol) {
            try {
              const matchStr = mol.get_substruct_match(qmol);
              if (matchStr) {
                const match = JSON.parse(matchStr);
                if (match.atoms && match.atoms.length > 0) {
                  options.atoms = match.atoms;
                  options.bonds = match.bonds || [];
                  const rgb = hexToRgb(highlightColor);
                  if (rgb) options.highlightColour = rgb;
                  hasOptions = true;
                  if (mounted) setHasSubstructMatch(true);
                }
              }
            } finally {
              qmol.delete();
            }
          }
        }

        // -- render SVG --
        let svg: string;
        if (hasOptions) {
          options.width = width;
          options.height = height;
          svg = mol.get_svg_with_highlights(JSON.stringify(options));
        } else {
          svg = mol.get_svg(width, height);
        }

        if (mounted) {
          setSvgContent(svg);
          setLoading(false);
          setTransform({ x: 0, y: 0, scale: 1 }); // reset zoom on new molecule
        }
      } catch (err) {
        console.error('MoleculeViewer render error:', err);
        if (mounted) {
          setError('Failed to render molecule');
          setLoading(false);
        }
      }
    };

    render();

    return () => {
      mounted = false;
      if (molRef.current) {
        molRef.current.delete();
        molRef.current = null;
      }
    };
  }, [smiles, width, height, substructure, highlightColor, atomColoring]);

  // --- Attach SVG event handlers (tooltip + pan) ---
  useEffect(() => {
    const wrapper = svgWrapperRef.current;
    if (!wrapper) return;

    const svgEl = wrapper.querySelector('svg');
    if (!svgEl) return;

    const getGroupIndex = (el: Element | null): { type: 'atom' | 'bond'; idx: number } | null => {
      while (el && el !== wrapper) {
        for (const cls of Array.from(el.classList)) {
          const atomMatch = /^atom-(\d+)$/.exec(cls);
          if (atomMatch) return { type: 'atom', idx: parseInt(atomMatch[1], 10) };
          const bondMatch = /^bond-(\d+)$/.exec(cls);
          if (bondMatch) return { type: 'bond', idx: parseInt(bondMatch[1], 10) };
        }
        el = el.parentElement;
      }
      return null;
    };

    const tooltipText = (type: 'atom' | 'bond', idx: number): string => {
      if (!molData) return type === 'atom' ? `Atom ${idx}` : `Bond ${idx}`;
      if (type === 'atom') {
        const a = molData.atoms[idx];
        if (!a) return `Atom ${idx}`;
        const parts = [a.symbol];
        if (a.formalCharge !== 0) parts.push(`${a.formalCharge > 0 ? '+' : '-'}${Math.abs(a.formalCharge)}`);
        parts.push(`(#${a.atomicNum})`);
        return parts.join(' ');
      }
      // bond
      const b = molData.bonds[idx];
      if (!b) return `Bond ${idx}`;
      const begin = molData.atoms[b.beginAtomIdx]?.symbol ?? `#${b.beginAtomIdx}`;
      const end = molData.atoms[b.endAtomIdx]?.symbol ?? `#${b.endAtomIdx}`;
      const label = BOND_TYPE_LABELS[b.bondType] ?? `Type ${b.bondType}`;
      return `${label}: ${begin}–${end}`;
    };

    const handleMouseMove = (e: MouseEvent) => {
      const rect = wrapper.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;

      // While dragging → pan; hide tooltip
      if (dragState.current.active) {
        setTooltip(null);
        const ds = dragState.current;
        const dx = mx - ds.startX;
        const dy = my - ds.startY;
        setTransform({
          x: ds.startTransform.x + dx,
          y: ds.startTransform.y + dy,
          scale: ds.startTransform.scale,
        });
        return;
      }

      // Tooltip update
      const target = e.target as Element;
      const info = getGroupIndex(target);
      if (info) {
        const text = tooltipText(info.type, info.idx);
        setTooltip({ type: info.type, text, x: mx + 12, y: my + 12 });
      } else {
        setTooltip(null);
      }
    };

    const handleMouseDown = (e: MouseEvent) => {
      // Ignore if user clicked on a toolbar button
      if ((e.target as Element).closest('[data-viewer-toolbar]')) return;

      dragState.current = {
        active: true,
        startX: e.clientX - wrapper.getBoundingClientRect().left,
        startY: e.clientY - wrapper.getBoundingClientRect().top,
        startTransform: { ...transform },
      };

      // Prevent text selection
      e.preventDefault();
    };

    const handleMouseUp = (e: MouseEvent) => {
      if (!dragState.current.active) return;
      dragState.current.active = false;

      // If the mouse barely moved, treat as a click
      const rect = wrapper.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;
      const ds = dragState.current;
      const dist = Math.sqrt((mx - ds.startX) ** 2 + (my - ds.startY) ** 2);
      if (dist < 4) {
        const target = e.target as Element;
        const info = getGroupIndex(target);
        if (info) {
          if (info.type === 'atom' && onAtomClick && molData) {
            const a = molData.atoms[info.idx];
            if (a) onAtomClick(info.idx, a.symbol);
          } else if (info.type === 'bond' && onBondClick && molData) {
            const b = molData.bonds[info.idx];
            if (b) onBondClick(info.idx, b.beginAtomIdx, b.endAtomIdx);
          }
        }
      }
    };

    const handleWheel = (e: WheelEvent) => {
      e.preventDefault();
      const rect = wrapper.getBoundingClientRect();
      const mx = e.clientX - rect.left;
      const my = e.clientY - rect.top;

      setTransform((prev) => {
        const factor = e.deltaY > 0 ? 0.85 : 1.18;
        const newScale = Math.max(0.1, Math.min(10, prev.scale * factor));
        const newX = mx - ((mx - prev.x) / prev.scale) * newScale;
        const newY = my - ((my - prev.y) / prev.scale) * newScale;
        return { x: newX, y: newY, scale: newScale };
      });
    };

    const handleMouseLeave = () => {
      setTooltip(null);
      dragState.current.active = false;
    };

    wrapper.addEventListener('mousemove', handleMouseMove);
    wrapper.addEventListener('mousedown', handleMouseDown);
    window.addEventListener('mouseup', handleMouseUp);
    wrapper.addEventListener('wheel', handleWheel, { passive: false });
    wrapper.addEventListener('mouseleave', handleMouseLeave);

    return () => {
      wrapper.removeEventListener('mousemove', handleMouseMove);
      wrapper.removeEventListener('mousedown', handleMouseDown);
      window.removeEventListener('mouseup', handleMouseUp);
      wrapper.removeEventListener('wheel', handleWheel);
      wrapper.removeEventListener('mouseleave', handleMouseLeave);
    };
  }, [svgContent, molData, transform, onAtomClick, onBondClick]);

  // =====================================================================
  // Render
  // =====================================================================

  // -- loading --
  if (loading) {
    return (
      <div
        className={`flex justify-center items-center bg-slate-50 rounded ${className}`}
        style={{ width, height }}
      >
        <LoadingSpinner size="sm" />
      </div>
    );
  }

  // -- error --
  if (error) {
    return (
      <div
        className={`flex justify-center items-center bg-red-50 text-red-400 text-xs rounded ${className}`}
        style={{ width, height }}
      >
        {error}
      </div>
    );
  }

  // -- empty --
  if (!svgContent) {
    return (
      <div
        className={`flex justify-center items-center bg-slate-50 text-slate-300 text-xs rounded ${className}`}
        style={{ width, height }}
      >
        No structure
      </div>
    );
  }

  // -- main render --
  return (
    <div
      className={`relative select-none rounded overflow-hidden bg-white ${className}`}
      style={{ width, height }}
    >
      {/* SVG canvas with pan / zoom wrapper */}
      <div
        ref={containerRef}
        className="absolute inset-0 overflow-hidden"
      >
        <div
          ref={svgWrapperRef}
          className="absolute inset-0 origin-top-left cursor-grab active:cursor-grabbing"
          style={{
            transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})`,
          }}
          dangerouslySetInnerHTML={{ __html: svgContent }}
        />
      </div>

      {/* Substructure-legend badge */}
      {hasSubstructMatch && substructure && (
        <div className="absolute bottom-2 left-2 bg-white/80 backdrop-blur text-[10px] text-slate-500 px-1.5 py-0.5 rounded border border-slate-200 leading-tight">
          Match: <span className="font-mono text-slate-700">{substructure}</span>
        </div>
      )}

      {/* Toolbar */}
      {showControls && (
        <div
          data-viewer-toolbar
          className="absolute top-1 right-1 flex gap-px opacity-60 hover:opacity-100 transition-opacity"
        >
          <button
            type="button"
            title="Zoom in"
            className="p-1 bg-white/80 backdrop-blur rounded-l border border-slate-200 hover:bg-slate-100 text-slate-600"
            onClick={zoomIn}
            aria-label="Zoom in"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
              <line x1="11" y1="8" x2="11" y2="14" /><line x1="8" y1="11" x2="14" y2="11" />
            </svg>
          </button>
          <button
            type="button"
            title="Zoom out"
            className="p-1 bg-white/80 backdrop-blur border border-slate-200 hover:bg-slate-100 text-slate-600"
            onClick={zoomOut}
            aria-label="Zoom out"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
              <line x1="8" y1="11" x2="14" y2="11" />
            </svg>
          </button>
          <button
            type="button"
            title="Reset view"
            className="p-1 bg-white/80 backdrop-blur border border-slate-200 hover:bg-slate-100 text-slate-600"
            onClick={resetView}
            aria-label="Reset view"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
              <path d="M3 3v5h5" />
            </svg>
          </button>
          <button
            type="button"
            title="Export SVG"
            className="p-1 bg-white/80 backdrop-blur border border-slate-200 hover:bg-slate-100 text-slate-600"
            onClick={() => {
              const svgEl = svgWrapperRef.current?.querySelector('svg');
              if (!svgEl) return;
              const svgData = new XMLSerializer().serializeToString(svgEl);
              downloadBlob(
                new Blob([svgData], { type: 'image/svg+xml;charset=utf-8' }),
                'molecule.svg'
              );
            }}
            aria-label="Export SVG"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7 10 12 15 17 10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
          </button>
          <button
            type="button"
            title="Export PNG"
            className="p-1 bg-white/80 backdrop-blur rounded-r border border-slate-200 hover:bg-slate-100 text-slate-600"
            onClick={async () => {
              const svgEl = svgWrapperRef.current?.querySelector('svg');
              if (!svgEl) return;

              const svgData = new XMLSerializer().serializeToString(svgEl);
              const canvas = document.createElement('canvas');
              const ctx = canvas.getContext('2d');
              if (!ctx) return;

              const img = new Image();
              try {
                await new Promise<void>((resolve, reject) => {
                  img.onload = () => resolve();
                  img.onerror = reject;
                  img.src =
                    'data:image/svg+xml;base64,' +
                    btoa(unescape(encodeURIComponent(svgData)));
                });
              } catch {
                return; // silent fail — PNG export from SVG may not always work
              }

              const scale = 2; // Hi-DPI
              canvas.width = img.width * scale || width * scale;
              canvas.height = img.height * scale || height * scale;
              ctx.scale(scale, scale);
              ctx.drawImage(img, 0, 0);

              canvas.toBlob((blob) => {
                if (blob) downloadBlob(blob, 'molecule.png');
              }, 'image/png');
            }}
            aria-label="Export PNG"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
              <circle cx="8.5" cy="8.5" r="1.5" />
              <polyline points="21 15 16 10 5 21" />
            </svg>
          </button>
        </div>
      )}

      {/* Tooltip */}
      {tooltip && (
        <div
          className="absolute z-20 pointer-events-none bg-slate-800 text-white text-[11px] px-2 py-1 rounded shadow-sm whitespace-nowrap"
          style={{
            left: Math.min(tooltip.x, width - 120),
            top: Math.min(tooltip.y, height - 28),
          }}
        >
          {tooltip.text}
        </div>
      )}
    </div>
  );
};

export default MoleculeViewer;
