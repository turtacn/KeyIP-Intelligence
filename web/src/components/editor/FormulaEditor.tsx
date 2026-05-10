import React, { useState, useMemo, useCallback } from 'react';

// ---------------------------------------------------------------------------
// Atomic masses (IUPAC standard atomic weights, rounded)
// ---------------------------------------------------------------------------

const ATOMIC_MASSES: Record<string, number> = {
  H: 1.008,
  He: 4.003,
  Li: 6.941,
  Be: 9.012,
  B: 10.811,
  C: 12.011,
  N: 14.007,
  O: 15.999,
  F: 18.998,
  Ne: 20.180,
  Na: 22.990,
  Mg: 24.305,
  Al: 26.982,
  Si: 28.086,
  P: 30.974,
  S: 32.065,
  Cl: 35.453,
  Ar: 39.948,
  K: 39.098,
  Ca: 40.078,
  Sc: 44.956,
  Ti: 47.867,
  V: 50.942,
  Cr: 51.996,
  Mn: 54.938,
  Fe: 55.845,
  Co: 58.933,
  Ni: 58.693,
  Cu: 63.546,
  Zn: 65.380,
  Ga: 69.723,
  Ge: 72.630,
  As: 74.922,
  Se: 78.971,
  Br: 79.904,
  Kr: 83.798,
  Rb: 85.468,
  Sr: 87.620,
  Y: 88.906,
  Zr: 91.224,
  Nb: 92.906,
  Mo: 95.950,
  Tc: 98.000,
  Ru: 101.070,
  Rh: 102.906,
  Pd: 106.420,
  Ag: 107.868,
  Cd: 112.414,
  In: 114.818,
  Sn: 118.711,
  Sb: 121.760,
  Te: 127.600,
  I: 126.904,
  Xe: 131.294,
  Cs: 132.905,
  Ba: 137.328,
  La: 138.905,
  Ce: 140.116,
  Pr: 140.908,
  Nd: 144.243,
  Sm: 150.360,
  Eu: 151.964,
  Gd: 157.250,
  Tb: 158.925,
  Dy: 162.500,
  Ho: 164.930,
  Er: 167.259,
  Tm: 168.934,
  Yb: 173.045,
  Lu: 174.967,
  Hf: 178.490,
  Ta: 180.948,
  W: 183.840,
  Re: 186.207,
  Os: 190.230,
  Ir: 192.217,
  Pt: 195.084,
  Au: 196.967,
  Hg: 200.592,
  Tl: 204.383,
  Pb: 207.200,
  Bi: 208.980,
  Po: 209.000,
  At: 210.000,
  Rn: 222.000,
  Fr: 223.000,
  Ra: 226.000,
  Ac: 227.000,
  Th: 232.038,
  Pa: 231.036,
  U: 238.029,
  Np: 237.000,
  Pu: 244.000,
  Am: 243.000,
  Cm: 247.000,
  Bk: 247.000,
  Cf: 251.000,
  Es: 252.000,
  Fm: 257.000,
  Md: 258.000,
  No: 259.000,
  Lr: 262.000,
};

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ElementEntry {
  symbol: string;
  count: number;
  mass: number;
  contribution: number;
  percentage: number;
}

interface FormulaResult {
  elements: ElementEntry[];
  totalMass: number;
  valid: boolean;
  error?: string;
}

// ---------------------------------------------------------------------------
// Formula parser
// ---------------------------------------------------------------------------

function parseChemicalFormula(formula: string): FormulaResult {
  const cleaned = formula.replace(/\s/g, '');
  if (!cleaned) {
    return { elements: [], totalMass: 0, valid: true };
  }

  const rawElements: { symbol: string; count: number }[] = [];
  let i = 0;

  function parseGroup(): boolean {
    while (i < cleaned.length) {
      // Open parenthesis
      if (cleaned[i] === '(') {
        i++;
        const startLen = rawElements.length;
        if (!parseGroup()) return false;

        // Read multiplier
        let multStr = '';
        while (i < cleaned.length && /\d/.test(cleaned[i])) {
          multStr += cleaned[i];
          i++;
        }
        const multiplier = multStr ? parseInt(multStr, 10) : 1;

        // Apply multiplier to elements in this group
        for (let j = startLen; j < rawElements.length; j++) {
          rawElements[j].count *= multiplier;
        }
        continue;
      }

      // Close parenthesis
      if (cleaned[i] === ')') {
        i++;
        return true;
      }

      // Element symbol
      if (/[A-Za-z]/.test(cleaned[i])) {
        let symbol = cleaned[i];
        i++;
        if (i < cleaned.length && /[a-z]/.test(cleaned[i])) {
          symbol += cleaned[i];
          i++;
        }

        // Validate element
        if (!ATOMIC_MASSES[symbol]) {
          return false;
        }

        // Read count
        let countStr = '';
        while (i < cleaned.length && /\d/.test(cleaned[i])) {
          countStr += cleaned[i];
          i++;
        }
        const count = countStr ? parseInt(countStr, 10) : 1;
        rawElements.push({ symbol, count });
        continue;
      }

      // Skip middle dot / dot (hydrates)
      if (cleaned[i] === '·' || cleaned[i] === '.' || cleaned[i] === '•') {
        i++;
        continue;
      }

      // Unexpected character
      return false;
    }
    return true;
  }

  if (!parseGroup()) {
    return { elements: [], totalMass: 0, valid: false, error: 'Invalid formula syntax or unknown element' };
  }

  if (i !== cleaned.length) {
    return { elements: [], totalMass: 0, valid: false, error: 'Unexpected character in formula' };
  }

  // Combine elements with the same symbol
  const elementMap = new Map<string, number>();
  for (const entry of rawElements) {
    elementMap.set(entry.symbol, (elementMap.get(entry.symbol) || 0) + entry.count);
  }

  // Sort: C first, H second, then alphabetical
  const sortedElements = Array.from(elementMap.entries())
    .map(([symbol, count]) => ({ symbol, count }))
    .sort((a, b) => {
      if (a.symbol === 'C') return -1;
      if (b.symbol === 'C') return 1;
      if (a.symbol === 'H') return -1;
      if (b.symbol === 'H') return 1;
      return a.symbol.localeCompare(b.symbol);
    });

  // Calculate mass contributions
  const totalMass = sortedElements.reduce(
    (sum, el) => sum + el.count * (ATOMIC_MASSES[el.symbol] || 0),
    0,
  );

  const elements: ElementEntry[] = sortedElements.map((el) => {
    const mass = ATOMIC_MASSES[el.symbol] || 0;
    const contribution = el.count * mass;
    return {
      symbol: el.symbol,
      count: el.count,
      mass,
      contribution,
      percentage: totalMass > 0 ? (contribution / totalMass) * 100 : 0,
    };
  });

  return { elements, totalMass, valid: true };
}

// ---------------------------------------------------------------------------
// Element color mapping for display
// ---------------------------------------------------------------------------

const ELEMENT_COLORS: Record<string, string> = {
  H: 'bg-gray-200 text-gray-800',
  C: 'bg-slate-200 text-slate-800',
  N: 'bg-blue-100 text-blue-800',
  O: 'bg-red-100 text-red-800',
  F: 'bg-emerald-100 text-emerald-800',
  Cl: 'bg-emerald-100 text-emerald-800',
  Br: 'bg-red-100 text-red-800',
  I: 'bg-purple-100 text-purple-800',
  S: 'bg-amber-100 text-amber-800',
  P: 'bg-orange-100 text-orange-800',
  B: 'bg-teal-100 text-teal-800',
  Si: 'bg-teal-100 text-teal-800',
  Se: 'bg-teal-100 text-teal-800',
  Te: 'bg-teal-100 text-teal-800',
  Ir: 'bg-purple-100 text-purple-800',
  Pd: 'bg-purple-100 text-purple-800',
  Pt: 'bg-purple-100 text-purple-800',
  Ru: 'bg-purple-100 text-purple-800',
  Rh: 'bg-purple-100 text-purple-800',
  Os: 'bg-purple-100 text-purple-800',
  Re: 'bg-purple-100 text-purple-800',
  Fe: 'bg-amber-100 text-amber-800',
  Co: 'bg-rose-100 text-rose-800',
  Ni: 'bg-teal-100 text-teal-800',
  Cu: 'bg-amber-100 text-amber-800',
  Zn: 'bg-gray-200 text-gray-800',
};

function getElementColor(symbol: string): string {
  return ELEMENT_COLORS[symbol] || 'bg-slate-100 text-slate-700';
}

// ---------------------------------------------------------------------------
// Progress bar color
// ---------------------------------------------------------------------------

function getBarColor(symbol: string): string {
  switch (symbol) {
    case 'C': return 'bg-slate-500';
    case 'H': return 'bg-gray-400';
    case 'N': return 'bg-blue-500';
    case 'O': return 'bg-red-500';
    case 'S': return 'bg-amber-500';
    case 'P': return 'bg-orange-500';
    case 'F': case 'Cl': return 'bg-emerald-500';
    case 'Br': return 'bg-red-500';
    case 'I': return 'bg-purple-500';
    default: return 'bg-blue-500';
  }
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface FormulaEditorProps {
  initialFormula?: string;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const FormulaEditor: React.FC<FormulaEditorProps> = ({ initialFormula = '' }) => {
  const [formula, setFormula] = useState(initialFormula);
  const [error, setError] = useState<string | null>(null);

  const handleChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    // Allow only valid formula characters
    if (/^[A-Za-z0-9()·.•\s]*$/.test(value) || value === '') {
      setFormula(value);
      setError(null);
    } else {
      setError('Only element symbols, numbers, and parentheses allowed');
    }
  }, []);

  const result = useMemo(() => {
    if (!formula.trim()) {
      return { elements: [], totalMass: 0, valid: true } as FormulaResult;
    }
    const r = parseChemicalFormula(formula);
    if (!r.valid) {
      setError(r.error || 'Parse error');
    }
    return r;
  }, [formula]);

  return (
    <div className="space-y-4">
      {/* Formula Input */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
          Chemical Formula
        </label>
        <div className="relative">
          <input
            type="text"
            value={formula}
            onChange={handleChange}
            spellCheck={false}
            autoComplete="off"
            className={`w-full px-3 py-2 font-mono text-sm border rounded-lg focus:outline-none focus:ring-2 transition-colors
              ${error
                ? 'border-red-300 focus:ring-red-500 focus:border-red-500'
                : result.valid && formula.trim()
                  ? 'border-emerald-300 focus:ring-emerald-500 focus:border-emerald-500'
                  : 'border-slate-300 dark:border-slate-600 focus:ring-blue-500 focus:border-blue-500'
              }
              bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-200
              placeholder-slate-400 dark:placeholder-slate-500`}
            placeholder="e.g., C8H10N4O2 or Ir(C11H12N2)3"
          />
          {result.valid && formula.trim() && (
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-[10px] text-emerald-500 font-medium">
              Valid
            </span>
          )}
        </div>
        {error && (
          <p className="mt-1 text-xs text-red-500 dark:text-red-400">{error}</p>
        )}
      </div>

      {/* Results section */}
      {formula.trim() && result.valid && result.elements.length > 0 && (
        <>
          {/* Molecular Weight */}
          <div className="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg p-4">
            <div className="text-xs text-slate-500 dark:text-slate-400 mb-0.5">
              Molecular Weight
            </div>
            <div className="text-2xl font-bold text-slate-800 dark:text-slate-100 tabular-nums">
              {result.totalMass.toFixed(4)}{' '}
              <span className="text-sm font-normal text-slate-400 dark:text-slate-500">g/mol</span>
            </div>
          </div>

          {/* Elemental Composition */}
          <div>
            <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">
              Elemental Composition
            </label>
            <div className="space-y-2">
              {result.elements.map((el) => (
                <div key={el.symbol} className="group">
                  <div className="flex items-center justify-between mb-1">
                    <div className="flex items-center gap-2">
                      <span
                        className={`inline-flex items-center justify-center w-7 h-7 rounded-md text-xs font-bold ${getElementColor(el.symbol)}`}
                      >
                        {el.symbol}
                      </span>
                      <span className="text-sm text-slate-600 dark:text-slate-400">
                        ×{el.count}
                      </span>
                      <span className="text-[11px] text-slate-400 dark:text-slate-500">
                        {el.mass.toFixed(3)} g/mol
                      </span>
                    </div>
                    <div className="text-right">
                      <span className="text-sm font-semibold text-slate-700 dark:text-slate-300 tabular-nums">
                        {el.percentage.toFixed(1)}%
                      </span>
                      <span className="text-[11px] text-slate-400 dark:text-slate-500 ml-1.5">
                        {el.contribution.toFixed(2)} g/mol
                      </span>
                    </div>
                  </div>
                  {/* Progress bar */}
                  <div className="w-full h-2 bg-slate-100 dark:bg-slate-700 rounded-full overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all duration-300 ${getBarColor(el.symbol)}`}
                      style={{ width: `${el.percentage}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Formula badge / summary */}
          <div className="flex flex-wrap items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
            <span className="font-medium text-slate-600 dark:text-slate-300">Formula:</span>
            {result.elements.map((el) => (
              <span key={el.symbol} className="inline-flex items-center gap-0.5">
                <span className={`font-bold ${getElementColor(el.symbol).split(' ')[1]}`}>
                  {el.symbol}
                </span>
                {el.count > 1 && (
                  <span className="text-slate-400 dark:text-slate-500">{el.count}</span>
                )}
              </span>
            ))}
          </div>
        </>
      )}

      {/* Empty state */}
      {!formula.trim() && (
        <div className="flex flex-col items-center justify-center py-8 text-slate-400 dark:text-slate-500">
          <svg className="w-10 h-10 mb-2 opacity-40" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M12 2L2 7l10 5 10-5-10-5z" />
            <path d="M2 17l10 5 10-5" />
            <path d="M2 12l10 5 10-5" />
          </svg>
          <p className="text-sm">Enter a chemical formula to calculate molecular weight</p>
          <p className="text-xs mt-1">Supports parentheses and multipliers, e.g., Ir(C11H12N2)3</p>
        </div>
      )}
    </div>
  );
};

export default FormulaEditor;
