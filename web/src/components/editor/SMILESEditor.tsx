import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { getRDKit, RDKitModule } from '../../utils/rdkitLoader';
import MoleculeViewer from '../ui/MoleculeViewer';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const HISTORY_KEY = 'smiles_editor_history';
const MAX_HISTORY = 10;

const FRAGMENTS: { name: string; smiles: string; formula: string; description: string }[] = [
  {
    name: 'Carbazole',
    smiles: 'c1ccc2c(c1)c3ccccc3[nH]2',
    formula: 'C12H9N',
    description: 'Common hole-transporting building block',
  },
  {
    name: 'Triazine',
    smiles: 'c1ncncn1',
    formula: 'C3H3N3',
    description: 'Electron-transport / host core',
  },
  {
    name: 'Fluorene',
    smiles: 'c1ccc2c(c1)Cc1ccccc1-2',
    formula: 'C13H10',
    description: 'Blue emitter / host backbone',
  },
  {
    name: 'Spiro',
    smiles: 'C12CCCCC1CCC2',
    formula: 'C10H18',
    description: 'Spiro-skeleton for rigid architectures',
  },
  {
    name: 'Ir',
    smiles: '[Ir]',
    formula: 'Ir',
    description: 'Iridium atom for phosphorescent complexes',
  },
];

// Atom colour mapping (Tailwind classes)
const ATOM_COLORS: Record<string, string> = {
  C: 'text-slate-800 dark:text-slate-200 font-semibold',
  c: 'text-slate-800 dark:text-slate-200 font-semibold',
  N: 'text-blue-600 dark:text-blue-400 font-semibold',
  n: 'text-blue-600 dark:text-blue-400 font-semibold',
  O: 'text-red-600 dark:text-red-400 font-semibold',
  o: 'text-red-600 dark:text-red-400 font-semibold',
  S: 'text-amber-600 dark:text-amber-400 font-semibold',
  s: 'text-amber-600 dark:text-amber-400 font-semibold',
  P: 'text-orange-600 dark:text-orange-400 font-semibold',
  p: 'text-orange-600 dark:text-orange-400 font-semibold',
  F: 'text-emerald-600 dark:text-emerald-400 font-semibold',
  f: 'text-emerald-600 dark:text-emerald-400 font-semibold',
  Cl: 'text-emerald-600 dark:text-emerald-400 font-semibold',
  Br: 'text-emerald-600 dark:text-emerald-400 font-semibold',
  I: 'text-emerald-600 dark:text-emerald-400 font-semibold',
  B: 'text-teal-600 dark:text-teal-400 font-semibold',
  b: 'text-teal-600 dark:text-teal-400 font-semibold',
  Si: 'text-teal-600 dark:text-teal-400 font-semibold',
  Se: 'text-teal-600 dark:text-teal-400 font-semibold',
  Te: 'text-teal-600 dark:text-teal-400 font-semibold',
  As: 'text-teal-600 dark:text-teal-400 font-semibold',
};

const METAL_PATTERN = /Ir|Pd|Pt|Ru|Rh|Os|Re|Au|Ag|Fe|Co|Ni|Cu|Zn|Mn|Cr|Ti|Zr|Hf|V|Nb|Ta|Mo|W|Tc|La|Ce|Pr|Nd|Sm|Eu|Gd|Tb|Dy|Ho|Er|Tm|Yb|Lu|Al|Ga|In|Sn|Sb|Bi|Pb|Hg|Cd/i;

// Two-letter element symbols
const TWO_LETTER_ATOMS = new Set([
  'Cl', 'Br', 'Si', 'Se', 'Te', 'As', 'He', 'Li', 'Be', 'Na', 'Mg', 'Al',
  'Ca', 'Sc', 'Ti', 'V', 'Cr', 'Mn', 'Fe', 'Co', 'Ni', 'Cu', 'Zn', 'Ga',
  'Ge', 'Kr', 'Rb', 'Sr', 'Y', 'Zr', 'Nb', 'Mo', 'Tc', 'Ru', 'Rh', 'Pd',
  'Ag', 'Cd', 'In', 'Sn', 'Sb', 'Cs', 'Ba', 'La', 'Ce', 'Pr', 'Nd', 'Pm',
  'Sm', 'Eu', 'Gd', 'Tb', 'Dy', 'Ho', 'Er', 'Tm', 'Yb', 'Lu', 'Hf', 'Ta',
  'W', 'Re', 'Os', 'Ir', 'Pt', 'Au', 'Hg', 'Tl', 'Pb', 'Bi', 'Po', 'At',
  'Rn', 'Fr', 'Ra', 'Ac', 'Th', 'Pa', 'U', 'Np', 'Pu', 'Am', 'Cm', 'Bk',
  'Cf', 'Es', 'Fm', 'Md', 'No', 'Lr', 'Rf', 'Db', 'Sg', 'Bh', 'Hs', 'Mt',
  'Ds', 'Rg', 'Cn', 'Nh', 'Fl', 'Mc', 'Lv', 'Ts', 'Og',
]);

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface TokenSpan {
  text: string;
  className: string;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Tokenize a SMILES string into coloured spans. */
function tokenizeSMILES(smiles: string): TokenSpan[] {
  const tokens: TokenSpan[] = [];
  let i = 0;

  while (i < smiles.length) {
    // Bracket expression [...]
    if (smiles[i] === '[') {
      const end = smiles.indexOf(']', i + 1);
      if (end === -1) {
        tokens.push({ text: smiles.slice(i), className: 'text-red-500 bg-red-50 dark:bg-red-900/20' });
        break;
      }
      const content = smiles.slice(i + 1, end);
      const isMetal = METAL_PATTERN.test(content);
      tokens.push({
        text: smiles.slice(i, end + 1),
        className: isMetal
          ? 'text-purple-600 dark:text-purple-400 font-semibold'
          : 'text-teal-600 dark:text-teal-400 font-semibold',
      });
      i = end + 1;
      continue;
    }

    const ch = smiles[i];

    // Two-letter atoms (e.g., Cl, Br, Si)
    if (i + 1 < smiles.length) {
      const two = smiles.slice(i, i + 2);
      if (TWO_LETTER_ATOMS.has(two)) {
        tokens.push({ text: two, className: ATOM_COLORS[two] ?? 'text-slate-800 dark:text-slate-200 font-semibold' });
        i += 2;
        continue;
      }
    }

    // Single letter: atom symbol (upper or lower)
    if (/[A-Za-z]/.test(ch)) {
      tokens.push({
        text: ch,
        className: ATOM_COLORS[ch] ?? 'text-slate-800 dark:text-slate-200 font-semibold',
      });
      i++;
      continue;
    }

    // Digits (ring closure numbers)
    if (/\d/.test(ch)) {
      let num = '';
      while (i < smiles.length && /\d/.test(smiles[i])) {
        num += smiles[i];
        i++;
      }
      tokens.push({ text: num, className: 'text-slate-500 dark:text-slate-400' });
      continue;
    }

    // Bonds
    if (/[-=#:/\\]/.test(ch)) {
      tokens.push({ text: ch, className: 'text-slate-400 dark:text-slate-500' });
      i++;
      continue;
    }

    // Dot (disconnection)
    if (ch === '.') {
      tokens.push({ text: ch, className: 'text-slate-300 dark:text-slate-600' });
      i++;
      continue;
    }

    // Percent sign (ring closure > 9)
    if (ch === '%') {
      tokens.push({ text: ch, className: 'text-slate-500 dark:text-slate-400' });
      i++;
      continue;
    }

    // Branches
    if (ch === '(' || ch === ')') {
      tokens.push({ text: ch, className: 'text-slate-500 dark:text-slate-400' });
      i++;
      continue;
    }

    // Charge
    if (ch === '+' || ch === '-') {
      tokens.push({ text: ch, className: 'text-orange-500 dark:text-orange-400 font-semibold' });
      i++;
      continue;
    }

    // Chirality
    if (ch === '@') {
      tokens.push({ text: ch, className: 'text-cyan-500 dark:text-cyan-400' });
      i++;
      continue;
    }

    // Anything else
    tokens.push({ text: ch, className: 'text-red-400' });
    i++;
  }

  return tokens;
}

/** Basic SMILES validation with position tracking. */
interface ValidationResult {
  valid: boolean;
  error?: string;
  position?: number;
}

function validateSMILES(smiles: string): ValidationResult {
  if (!smiles.trim()) return { valid: true };

  // Check balanced parentheses
  let parenDepth = 0;
  for (let i = 0; i < smiles.length; i++) {
    if (smiles[i] === '(') parenDepth++;
    if (smiles[i] === ')') {
      parenDepth--;
      if (parenDepth < 0) {
        return { valid: false, error: 'Unmatched closing parenthesis', position: i };
      }
    }
  }
  if (parenDepth > 0) {
    return { valid: false, error: 'Unmatched opening parenthesis', position: smiles.lastIndexOf('(') };
  }

  // Check balanced brackets
  let bracketDepth = 0;
  for (let i = 0; i < smiles.length; i++) {
    if (smiles[i] === '[') bracketDepth++;
    if (smiles[i] === ']') {
      bracketDepth--;
      if (bracketDepth < 0) {
        return { valid: false, error: 'Unmatched closing bracket', position: i };
      }
    }
  }
  if (bracketDepth > 0) {
    return { valid: false, error: 'Unmatched opening bracket', position: smiles.lastIndexOf('[') };
  }

  // Empty brackets
  const emptyBracket = smiles.match(/\[\s*\]/);
  if (emptyBracket && emptyBracket.index !== undefined) {
    return { valid: false, error: 'Empty brackets []', position: emptyBracket.index };
  }

  return { valid: true };
}

/** Load history from localStorage. */
function loadHistory(): string[] {
  try {
    const data = localStorage.getItem(HISTORY_KEY);
    return data ? JSON.parse(data) : [];
  } catch {
    return [];
  }
}

/** Save a SMILES to history (dedup, max 10). */
function saveToHistory(history: string[], smiles: string): string[] {
  if (!smiles.trim()) return history;
  const next = [smiles, ...history.filter((s) => s !== smiles)].slice(0, MAX_HISTORY);
  localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
  return next;
}

/** Remove a SMILES from history. */
function removeFromHistory(history: string[], smiles: string): string[] {
  const next = history.filter((s) => s !== smiles);
  localStorage.setItem(HISTORY_KEY, JSON.stringify(next));
  return next;
}

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface SMILESEditorProps {
  /** Initial SMILES value */
  initialSmiles?: string;
  /** Called when a valid SMILES is committed */
  onChange?: (smiles: string) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const SMILESEditor: React.FC<SMILESEditorProps> = ({ initialSmiles = '', onChange }) => {
  // --- state ---
  const [smiles, setSmiles] = useState(initialSmiles);
  const [error, setError] = useState<ValidationResult | null>(null);
  const [rdkitError, setRdkitError] = useState<string | null>(null);
  const [debouncedSmiles, setDebouncedSmiles] = useState(initialSmiles);
  const [history, setHistory] = useState<string[]>(loadHistory);
  const [rdkitReady, setRdkitReady] = useState(false);

  // --- refs ---
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlightRef = useRef<HTMLPreElement>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();

  // --- Init RDKit ---
  useEffect(() => {
    getRDKit()
      .then(() => setRdkitReady(true))
      .catch(() => setRdkitReady(false));
  }, []);

  // --- Debounce SMILES for preview ---
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setDebouncedSmiles(smiles);
    }, 300);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [smiles]);

  // --- Validate on every change ---
  useEffect(() => {
    if (!smiles.trim()) {
      setError(null);
      setRdkitError(null);
      return;
    }

    const result = validateSMILES(smiles);
    setError(result.valid ? null : result);

    // RDKit validation (only if basic validation passes)
    if (result.valid && rdkitReady) {
      getRDKit()
        .then((rdkit: RDKitModule) => {
          try {
            const mol = rdkit.get_mol(smiles);
            if (!mol) {
              setRdkitError('RDKit could not parse this SMILES');
            } else {
              setRdkitError(null);
              mol.delete();
            }
          } catch (e: unknown) {
            const msg = e instanceof Error ? e.message : 'Unknown error';
            setRdkitError(`RDKit error: ${msg}`);
          }
        })
        .catch(() => {
          // RDKit not available — skip server-side validation
        });
    } else {
      setRdkitError(null);
    }
  }, [smiles, rdkitReady]);

  // --- Scroll sync ---
  const handleScroll = useCallback(() => {
    if (highlightRef.current && textareaRef.current) {
      highlightRef.current.scrollTop = textareaRef.current.scrollTop;
      highlightRef.current.scrollLeft = textareaRef.current.scrollLeft;
    }
  }, []);

  // --- Handlers ---
  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setSmiles(e.target.value);
  }, []);

  const handleInsertFragment = useCallback((fragmentSmiles: string) => {
    const ta = textareaRef.current;
    if (ta) {
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const before = smiles.slice(0, start);
      const after = smiles.slice(end);
      // Insert with a dot if both sides are non-empty
      const prefix = before && !before.endsWith('.') ? '.' : '';
      const suffix = after && !after.startsWith('.') ? '.' : '';
      const newSmiles = before + prefix + fragmentSmiles + suffix + after;
      setSmiles(newSmiles);
      // Notify parent
      onChange?.(newSmiles);
      // Focus back
      requestAnimationFrame(() => {
        ta.focus();
        const pos = start + prefix.length + fragmentSmiles.length;
        ta.setSelectionRange(pos, pos);
      });
    } else {
      // Fallback: append
      const sep = smiles ? '.' : '';
      const newSmiles = smiles + sep + fragmentSmiles;
      setSmiles(newSmiles);
      onChange?.(newSmiles);
    }
  }, [smiles, onChange]);

  const handleHistorySelect = useCallback((s: string) => {
    setSmiles(s);
    onChange?.(s);
  }, [onChange]);

  const handleHistoryRemove = useCallback((s: string) => {
    setHistory((prev) => removeFromHistory(prev, s));
  }, []);

  const handleSave = useCallback(() => {
    if (smiles.trim() && !error && !rdkitError) {
      setHistory((prev) => saveToHistory(prev, smiles));
    }
  }, [smiles, error, rdkitError]);

  const handleClear = useCallback(() => {
    setSmiles('');
    setError(null);
    setRdkitError(null);
    onChange?.('');
  }, [onChange]);

  // --- Tokenized highlight ---
  const tokens = useMemo(() => tokenizeSMILES(smiles), [smiles]);
  const displayError = error || (rdkitError ? { valid: false, error: rdkitError } : null);

  // =====================================================================
  // Render
  // =====================================================================

  return (
    <div className="space-y-4">
      {/* SMILES Input with syntax highlighting */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
          SMILES Input
        </label>
        <div className="relative border border-slate-300 dark:border-slate-600 rounded-lg overflow-hidden focus-within:ring-2 focus-within:ring-blue-500 focus-within:border-blue-500">
          {/* Highlighted overlay */}
          <pre
            ref={highlightRef}
            className="absolute inset-0 m-0 p-3 font-mono text-sm leading-5 whitespace-pre-wrap break-all pointer-events-none overflow-hidden"
            aria-hidden="true"
          >
            <code>
              {tokens.map((t, i) => (
                <span key={i} className={t.className}>
                  {t.text}
                </span>
              ))}
              {/* Trailing newline for alignment */}
              {'\n'}
            </code>
          </pre>
          {/* Actual textarea */}
          <textarea
            ref={textareaRef}
            value={smiles}
            onChange={handleChange}
            onScroll={handleScroll}
            onBlur={handleSave}
            spellCheck={false}
            autoComplete="off"
            autoCapitalize="off"
            autoCorrect="off"
            rows={3}
            className="relative w-full p-3 font-mono text-sm leading-5 whitespace-pre-wrap break-all bg-transparent text-transparent caret-slate-800 dark:caret-slate-200 resize-y focus:outline-none"
            placeholder="Enter SMILES, e.g., c1ccc2c(c1)c3ccccc3[nH]2"
          />
        </div>

        {/* Error indicator */}
        {displayError && smiles.trim() && (
          <div className="mt-1.5 flex items-start gap-2 text-xs text-red-600 dark:text-red-400">
            <svg className="w-4 h-4 mt-0.5 flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <div>
              <span className="font-medium">Error:</span>{' '}
              {displayError.error}
              {displayError.position !== undefined && (
                <span className="ml-1 text-slate-500">
                  (at position {displayError.position + 1})
                </span>
              )}
              {/* Position marker */}
              {displayError.position !== undefined && (
                <div className="mt-0.5 font-mono text-slate-400 dark:text-slate-500 leading-4">
                  {smiles.slice(0, displayError.position + 1)}
                  <span className="text-red-500 font-bold">▲</span>
                  {smiles.slice(displayError.position + 1)}
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* 2D Preview */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
          2D Structure Preview
        </label>
        <div className="border border-slate-200 dark:border-slate-600 rounded-lg overflow-hidden bg-white">
          <MoleculeViewer
            smiles={debouncedSmiles}
            width={400}
            height={240}
            atomColoring
            showControls
          />
        </div>
      </div>

      {/* Common Fragments */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
          Common Fragments
        </label>
        <div className="flex flex-wrap gap-2">
          {FRAGMENTS.map((frag) => (
            <button
              key={frag.name}
              type="button"
              onClick={() => handleInsertFragment(frag.smiles)}
              title={`${frag.name} — ${frag.description}`}
              className="group relative inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-medium rounded-md border border-slate-200 dark:border-slate-600 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-300 hover:bg-blue-50 hover:border-blue-300 dark:hover:bg-blue-900/20 dark:hover:border-blue-500 transition-colors"
            >
              <span className="font-semibold">{frag.name}</span>
              <span className="text-slate-400 dark:text-slate-500 text-[10px]">
                {frag.formula}
              </span>
              {/* Tooltip */}
              <span className="absolute -top-8 left-1/2 -translate-x-1/2 px-2 py-1 bg-slate-800 text-white text-[10px] rounded shadow-sm opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none whitespace-nowrap z-10">
                {frag.description}
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* Action buttons */}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={handleClear}
          className="px-3 py-1.5 text-xs font-medium rounded-md border border-slate-300 dark:border-slate-600 text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors"
        >
          Clear
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={!smiles.trim() || !!error || !!rdkitError}
          className="px-3 py-1.5 text-xs font-medium rounded-md bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          Save to History
        </button>
      </div>

      {/* History */}
      <div>
        <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
          Recent SMILES History
        </label>
        {history.length === 0 ? (
          <p className="text-xs text-slate-400 dark:text-slate-500 italic">
            No recent SMILES. Valid SMILES are saved automatically on blur.
          </p>
        ) : (
          <div className="space-y-1 max-h-40 overflow-y-auto">
            {history.map((s, i) => (
              <div
                key={`${s}-${i}`}
                className="group flex items-center gap-2 px-2.5 py-1.5 rounded-md hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors cursor-pointer"
                onClick={() => handleHistorySelect(s)}
              >
                <span className="text-[10px] text-slate-400 dark:text-slate-500 w-4 flex-shrink-0">
                  {i + 1}
                </span>
                <code className="flex-1 text-xs font-mono text-slate-700 dark:text-slate-300 truncate">
                  {s}
                </code>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleHistoryRemove(s);
                  }}
                  className="opacity-0 group-hover:opacity-100 p-0.5 text-slate-400 hover:text-red-500 transition-all"
                  title="Remove from history"
                >
                  <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default SMILESEditor;
