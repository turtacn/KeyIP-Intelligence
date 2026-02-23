import React, { useEffect, useRef } from 'react';
import SmilesDrawer from 'smiles-drawer';

interface MoleculeViewerProps {
  smiles: string;
  width?: number;
  height?: number;
  className?: string;
  theme?: 'light' | 'dark';
}

const MoleculeViewer: React.FC<MoleculeViewerProps> = ({
  smiles,
  width = 300,
  height = 200,
  className = '',
  theme = 'light'
}) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    if (!canvasRef.current || !smiles) return;

    const drawer = new SmilesDrawer.Drawer({
      width: width,
      height: height,
      compactDrawing: false,
    });

    SmilesDrawer.parse(smiles, (tree: any) => {
      drawer.draw(tree, canvasRef.current, theme, false);
    }, (err: any) => {
      console.error('Error parsing SMILES:', err);
    });

  }, [smiles, width, height, theme]);

  return (
    <div className={`flex justify-center items-center ${className}`}>
      <canvas
        ref={canvasRef}
        width={width}
        height={height}
        className="max-w-full h-auto"
      />
    </div>
  );
};

export default MoleculeViewer;
