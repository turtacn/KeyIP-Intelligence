export interface RDKitModule {
  get_mol: (smiles: string) => RDKitMolecule | null;
  version: () => string;
}

export interface RDKitMolecule {
  get_svg: (width?: number, height?: number) => string;
  get_svg_with_highlights: (details: string) => string;
  delete: () => void;
}

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
             // Check if it's a function directly or on .default
             const loader = module.default || module;

             // The loader might be the function itself
             if (typeof loader === 'function') {
                 // Important: RDKit JS often expects to find the .wasm file at a specific path
                 // We need to pass locateFile options if supported, or ensure it's in root.

                 // Type casting to any to bypass strict type check for now on loader signature
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
