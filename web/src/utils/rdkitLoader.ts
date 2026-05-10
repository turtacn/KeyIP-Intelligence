/**
 * RDKit Module loader and TypeScript type definitions.
 *
 * Provides a singleton async loader for the @rdkit/rdkit WASM module
 * and extends the base JSMol interface with methods needed by MoleculeViewer.
 */

export interface RDKitMolecule {
  delete(): void;

  // -- string representations --
  get_smiles(): string;
  get_cxsmiles(): string;
  get_smarts(): string;
  get_cxsmarts(): string;
  get_molblock(): string;
  get_v3Kmolblock(): string;
  get_inchi(): string;
  get_json(): string;

  // -- SVG depictions --
  get_svg(): string;
  get_svg(width: number, height: number): string;
  get_svg_with_highlights(details: string): string;

  // -- substructure search --
  get_substruct_match(q: RDKitMolecule): string;
  get_substruct_matches(q: RDKitMolecule): string;

  // -- molecular descriptors / fingerprints --
  get_descriptors(): string;
  get_morgan_fp(): string;
  get_morgan_fp(options: string): string;
  get_pattern_fp(): string;
  is_valid(): boolean;
  has_coords(): boolean;

  // -- coordinate generation --
  set_new_coords(): boolean;
  set_new_coords(useCoordGen: boolean): boolean;
  get_new_coords(): string;
  normalize_depiction(): number;
  straighten_depiction(): void;

  // -- properties --
  has_prop(key: string): boolean;
  get_prop_list(): string[];
  set_prop(key: string, val: string): boolean;
  get_prop(key: string): string;

  // -- canvas rendering --
  draw_to_canvas(canvas: HTMLCanvasElement, width: number, height: number): void;
  draw_to_canvas_with_highlights(canvas: HTMLCanvasElement, details: string): void;
}

export interface RDKitModule {
  get_mol(input: string, details_json?: string): RDKitMolecule | null;
  get_mol_copy(other: RDKitMolecule): RDKitMolecule;
  get_qmol(input: string): RDKitMolecule | null;
  version(): string;
  prefer_coordgen(prefer: boolean): void;
}

// ---------------------------------------------------------------------------
// JSON shapes returned by RDKit
// ---------------------------------------------------------------------------

export interface RDKitAtomJSON {
  atomicNum: number;
  symbol: string;
  formalCharge: number;
  isotope: number;
  hybridization: number;
}

export interface RDKitBondJSON {
  beginAtomIdx: number;
  endAtomIdx: number;
  bondType: number;
  stereo: number;
}

export interface RDKitMoleculeJSON {
  atoms: RDKitAtomJSON[];
  bonds: RDKitBondJSON[];
  stereo: unknown[];
  properties: Record<string, unknown>;
}

// ---------------------------------------------------------------------------
// Singleton loader
// ---------------------------------------------------------------------------

let rdkitPromise: Promise<RDKitModule> | null = null;

export const getRDKit = (): Promise<RDKitModule> => {
  if (!rdkitPromise) {
    rdkitPromise = new Promise((resolve, reject) => {
      // Prioritize global loader if available (e.g. script tag)
      if (window.initRDKitModule) {
        window.initRDKitModule()
          .then(resolve)
          .catch(reject);
      } else {
        // Fallback: Try dynamic import of npm package
        import('@rdkit/rdkit')
          .then((module) => {
             const loader = module.default || module;

             if (typeof loader === 'function') {
                 const initFn = loader as any;

                 initFn({
                    locateFile: (file: string) => {
                        if (file.endsWith('.wasm')) return '/' + file;
                        return file;
                    }
                 }).then((instance: any) => {
                     resolve(instance as RDKitModule);
                 }).catch((e: any) => {
                     console.error("RDKit initialization failed", e);
                     reject(e);
                 });
             } else {
                 reject(new Error('RDKit loader is not a function'));
             }
          })
          .catch((e) => {
             console.error("Failed to load RDKit module via import", e);
             reject(e);
          });
      }
    });
  }
  return rdkitPromise;
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** CPK element colours (RGB tuples, 0-1 range). */
export const CPK_COLORS: Record<string, [number, number, number]> = {
  H:  [1.0, 1.0, 1.0],
  C:  [0.2, 0.2, 0.2],
  N:  [0.0, 0.0, 1.0],
  O:  [1.0, 0.0, 0.0],
  F:  [0.0, 0.5, 0.0],
  Cl: [0.0, 1.0, 0.0],
  Br: [0.5, 0.0, 0.0],
  I:  [0.4, 0.0, 0.6],
  He: [0.0, 1.0, 1.0],
  Li: [0.5, 0.0, 0.0],
  Be: [0.0, 0.5, 0.0],
  B:  [0.0, 0.5, 0.0],
  Ne: [0.0, 1.0, 1.0],
  Na: [0.0, 0.0, 1.0],
  Mg: [0.0, 0.5, 0.0],
  Al: [0.5, 0.5, 0.5],
  Si: [0.5, 0.5, 0.5],
  P:  [1.0, 0.5, 0.0],
  S:  [1.0, 1.0, 0.0],
  K:  [0.5, 0.0, 0.5],
  Ca: [0.5, 0.0, 0.0],
  Mn: [0.0, 0.5, 0.5],
  Fe: [0.5, 0.5, 0.0],
  Cu: [0.5, 0.3, 0.0],
  Zn: [0.3, 0.5, 0.3],
};
