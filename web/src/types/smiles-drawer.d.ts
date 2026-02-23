declare module 'smiles-drawer' {
  export class Drawer {
    constructor(options: any);
    draw(tree: any, canvas: HTMLCanvasElement | null, theme: string, infoOnly: boolean): void;
  }
  export function parse(smiles: string, successCallback: (tree: any) => void, errorCallback: (err: any) => void): void;
}
