import { render, waitFor } from '@testing-library/react';
import MoleculeViewer from '../../../components/ui/MoleculeViewer';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as rdkitLoader from '../../../utils/rdkitLoader';

// Mock getRDKit
vi.mock('../../../utils/rdkitLoader', async (importOriginal) => {
  const actual = await importOriginal<typeof rdkitLoader>();
  return {
    ...actual,
    getRDKit: vi.fn(),
  };
});

const MOCK_MOL_JSON = JSON.stringify({
  atoms: [
    { atomicNum: 6, symbol: 'C', formalCharge: 0, isotope: 0, hybridization: 1 },
    { atomicNum: 6, symbol: 'C', formalCharge: 0, isotope: 0, hybridization: 1 },
  ],
  bonds: [
    { beginAtomIdx: 0, endAtomIdx: 1, bondType: 1, stereo: 0 },
  ],
  stereo: [],
  properties: {},
});

describe('MoleculeViewer', () => {
  let mockDelete: ReturnType<typeof vi.fn>;
  let mockGetMol: ReturnType<typeof vi.fn>;
  let mockGetSvg: ReturnType<typeof vi.fn>;
  let mockGetJson: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockDelete = vi.fn();
    mockGetJson = vi.fn().mockReturnValue(MOCK_MOL_JSON);
    mockGetSvg = vi.fn().mockReturnValue('<svg>structure</svg>');
    mockGetMol = vi.fn().mockReturnValue({
      get_svg: mockGetSvg,
      get_json: mockGetJson,
      delete: mockDelete,
      get_substruct_match: vi.fn().mockReturnValue(''),
      set_new_coords: vi.fn().mockReturnValue(true),
      has_coords: vi.fn().mockReturnValue(true),
      is_valid: vi.fn().mockReturnValue(true),
    });

    (rdkitLoader.getRDKit as any).mockResolvedValue({
      get_mol: mockGetMol,
      get_qmol: vi.fn().mockReturnValue(null),
      version: vi.fn().mockReturnValue('2025.03.4'),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('calls delete only once when unmounted after render', async () => {
    const { unmount } = render(<MoleculeViewer smiles="C" />);

    await waitFor(() => {
      expect(mockGetMol).toHaveBeenCalled();
    });

    // Wait for the effect to run and molecule to be created
    await waitFor(() => {
      expect(mockGetJson).toHaveBeenCalled();
    });

    // get_svg should have been called
    await waitFor(() => {
      expect(mockGetSvg).toHaveBeenCalled();
    });

    // delete should not have been called yet (molecule is still active)
    expect(mockDelete).not.toHaveBeenCalled();

    // Unmount the component — this triggers cleanup
    unmount();

    // delete should now have been called exactly once (from cleanup)
    expect(mockDelete).toHaveBeenCalledTimes(1);
  });

  it('renders loading state initially', () => {
    const { container } = render(<MoleculeViewer smiles="C" />);
    expect(container.querySelector('.animate-spin')).toBeTruthy();
  });

  it('shows error for empty/invalid SMILES when molecule is null', async () => {
    mockGetMol.mockReturnValue(null);
    const { container } = render(<MoleculeViewer smiles="invalid" />);

    await waitFor(() => {
      expect(container.textContent).toContain('Invalid SMILES');
    });
  });

  it('renders SVG content after loading', async () => {
    const { container } = render(<MoleculeViewer smiles="CCO" />);

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });
  });

  it('renders nothing when smiles is empty', async () => {
    const { container } = render(<MoleculeViewer smiles="" />);

    await waitFor(() => {
      expect(container.textContent).toContain('No structure');
    });
  });

  it('accepts substructure SMARTS prop', async () => {
    const mockGetQmol = vi.fn().mockReturnValue(null);
    (rdkitLoader.getRDKit as any).mockResolvedValue({
      get_mol: mockGetMol,
      get_qmol: mockGetQmol,
      version: vi.fn().mockReturnValue('2025.03.4'),
    });

    render(
      <MoleculeViewer smiles="CCO" substructure="CO" />
    );

    await waitFor(() => {
      expect(mockGetQmol).toHaveBeenCalledWith('CO');
    });
  });

  it('supports atomColoring prop', async () => {
    const { container } = render(
      <MoleculeViewer smiles="CCO" atomColoring />
    );

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });
  });

  it('accepts custom dimensions', async () => {
    const { container } = render(
      <MoleculeViewer smiles="C" width={400} height={300} />
    );

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    expect(mockGetSvg).toHaveBeenCalledWith(400, 300);
  });

  it('renders with showControls={false}', async () => {
    const { container } = render(
      <MoleculeViewer smiles="C" showControls={false} />
    );

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    // Toolbar should not be present
    const toolbar = container.querySelector('[data-viewer-toolbar]');
    expect(toolbar).toBeFalsy();
  });
});
