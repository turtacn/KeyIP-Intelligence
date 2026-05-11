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
  let mockGetSvgWithHighlights: ReturnType<typeof vi.fn>;
  let mockGetJson: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockDelete = vi.fn();
    mockGetJson = vi.fn().mockReturnValue(MOCK_MOL_JSON);
    mockGetSvg = vi.fn().mockReturnValue('<svg>structure</svg>');
    mockGetSvgWithHighlights = vi.fn().mockReturnValue('<svg>highlighted</svg>');
    mockGetMol = vi.fn().mockReturnValue({
      get_svg: mockGetSvg,
      get_svg_with_highlights: mockGetSvgWithHighlights,
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

  // ---- atomColoring modes ----

  it('calls get_svg_with_highlights when atomColoring is enabled with CPK colors', async () => {
    const { container } = render(<MoleculeViewer smiles="C" atomColoring />);

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    // C has CPK_COLORS entry → atomColours is non-empty → get_svg_with_highlights called
    expect(mockGetSvgWithHighlights).toHaveBeenCalled();
    expect(mockGetSvg).not.toHaveBeenCalled();
  });

  it('falls back to plain get_svg when no CPK colors match the molecule atoms', async () => {
    mockGetJson.mockReturnValue(JSON.stringify({
      atoms: [
        { atomicNum: 0, symbol: 'Xx', formalCharge: 0, isotope: 0, hybridization: 1 },
      ],
      bonds: [],
      stereo: [],
      properties: {},
    }));

    const { container } = render(<MoleculeViewer smiles="Xx" atomColoring />);

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    // 'Xx' has no CPK_COLORS entry → atomColours stays empty → plain get_svg used
    expect(mockGetSvg).toHaveBeenCalled();
    expect(mockGetSvgWithHighlights).not.toHaveBeenCalled();
  });

  it('calls get_svg (no highlights) when atomColoring is disabled', async () => {
    const { container } = render(<MoleculeViewer smiles="C" atomColoring={false} />);

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    expect(mockGetSvg).toHaveBeenCalled();
    expect(mockGetSvgWithHighlights).not.toHaveBeenCalled();
  });

  // ---- substructure highlighting ----

  it('shows substructure match badge when substructure is found', async () => {
    const mockQmolDelete = vi.fn();
    const mockGetQmol = vi.fn().mockReturnValue({ delete: mockQmolDelete });

    mockGetMol.mockReturnValue({
      get_svg: mockGetSvg,
      get_svg_with_highlights: vi.fn().mockReturnValue('<svg>highlighted</svg>'),
      get_json: mockGetJson,
      delete: mockDelete,
      get_substruct_match: vi
        .fn()
        .mockReturnValue(JSON.stringify({ atoms: [0], bonds: [0] })),
      set_new_coords: vi.fn().mockReturnValue(true),
      has_coords: vi.fn().mockReturnValue(true),
      is_valid: vi.fn().mockReturnValue(true),
    });

    (rdkitLoader.getRDKit as any).mockResolvedValue({
      get_mol: mockGetMol,
      get_qmol: mockGetQmol,
      version: vi.fn().mockReturnValue('2025.03.4'),
    });

    const { container } = render(
      <MoleculeViewer smiles="CCO" substructure="CO" />
    );

    await waitFor(() => {
      expect(container.textContent).toContain('Match:');
      expect(container.textContent).toContain('CO');
    });
  });

  it('does not show substructure badge when no match found', async () => {
    const mockQmolDelete = vi.fn();
    const mockGetQmol = vi.fn().mockReturnValue({ delete: mockQmolDelete });

    (rdkitLoader.getRDKit as any).mockResolvedValue({
      get_mol: mockGetMol,
      get_qmol: mockGetQmol,
      version: vi.fn().mockReturnValue('2025.03.4'),
    });

    const { container } = render(
      <MoleculeViewer smiles="CCO" substructure="CCCC" />
    );

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    expect(container.textContent).not.toContain('Match:');
  });

  it('calls get_svg_with_highlights when substructure matches', async () => {
    const mockQmolDelete = vi.fn();
    const mockGetQmol = vi.fn().mockReturnValue({ delete: mockQmolDelete });
    const mockGetSvgHighlight = vi.fn().mockReturnValue('<svg>highlighted</svg>');

    mockGetMol.mockReturnValue({
      get_svg: mockGetSvg,
      get_svg_with_highlights: mockGetSvgHighlight,
      get_json: mockGetJson,
      delete: mockDelete,
      get_substruct_match: vi
        .fn()
        .mockReturnValue(JSON.stringify({ atoms: [0], bonds: [] })),
      set_new_coords: vi.fn().mockReturnValue(true),
      has_coords: vi.fn().mockReturnValue(true),
      is_valid: vi.fn().mockReturnValue(true),
    });

    (rdkitLoader.getRDKit as any).mockResolvedValue({
      get_mol: mockGetMol,
      get_qmol: mockGetQmol,
      version: vi.fn().mockReturnValue('2025.03.4'),
    });

    render(<MoleculeViewer smiles="CCO" substructure="CO" />);

    await waitFor(() => {
      expect(mockGetSvgHighlight).toHaveBeenCalled();
    });

    expect(mockGetSvg).not.toHaveBeenCalled();
  });

  // ---- export buttons ----

  it('renders SVG and PNG export buttons in toolbar', async () => {
    const { container } = render(<MoleculeViewer smiles="C" />);

    await waitFor(() => {
      expect(container.querySelector('svg')).toBeTruthy();
    });

    expect(container.querySelector('[aria-label="Export SVG"]')).toBeTruthy();
    expect(container.querySelector('[aria-label="Export PNG"]')).toBeTruthy();
    expect(container.querySelector('[aria-label="Zoom in"]')).toBeTruthy();
    expect(container.querySelector('[aria-label="Zoom out"]')).toBeTruthy();
    expect(container.querySelector('[aria-label="Reset view"]')).toBeTruthy();
  });

  // ---- error states ----

  it('shows error when RDKit fails to load', async () => {
    (rdkitLoader.getRDKit as any).mockRejectedValue(new Error('Network error'));

    const { container } = render(<MoleculeViewer smiles="C" />);

    await waitFor(() => {
      expect(container.textContent).toContain('Failed to render molecule');
    });

    // Verify error styling with red background
    expect(container.querySelector('.bg-red-50')).toBeTruthy();
    expect(container.querySelector('.text-red-400')).toBeTruthy();
  });

  it('shows error when get_mol throws an exception', async () => {
    mockGetMol.mockImplementation(() => {
      throw new Error('Unexpected RDKit error');
    });

    const { container } = render(<MoleculeViewer smiles="C" />);

    await waitFor(() => {
      expect(container.textContent).toContain('Failed to render molecule');
    });
  });

  it('shows error state with correct dimensions', async () => {
    mockGetMol.mockReturnValue(null);

    const { container } = render(
      <MoleculeViewer smiles="invalid" width={500} height={300} />
    );

    await waitFor(() => {
      expect(container.textContent).toContain('Invalid SMILES');
    });

    const errorDiv = container.querySelector('.bg-red-50');
    expect(errorDiv).toBeTruthy();
    expect((errorDiv as HTMLElement).style.width).toBe('500px');
    expect((errorDiv as HTMLElement).style.height).toBe('300px');
  });
});
