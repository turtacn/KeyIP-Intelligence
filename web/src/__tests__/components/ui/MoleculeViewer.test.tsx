import { render, waitFor } from '@testing-library/react';
import MoleculeViewer from '../../../components/ui/MoleculeViewer';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as rdkitLoader from '../../../utils/rdkitLoader';

// Mock getRDKit
vi.mock('../../../utils/rdkitLoader', () => ({
  getRDKit: vi.fn(),
}));

describe('MoleculeViewer Reproduction', () => {
  let mockDelete: any;
  let mockGetMol: any;
  let mockGetSvg: any;

  beforeEach(() => {
    mockDelete = vi.fn();
    mockGetSvg = vi.fn().mockReturnValue('<svg>structure</svg>');
    mockGetMol = vi.fn().mockReturnValue({
      get_svg: mockGetSvg,
      delete: mockDelete,
    });

    (rdkitLoader.getRDKit as any).mockResolvedValue({
      get_mol: mockGetMol,
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('calls delete only once when unmounted after render', async () => {
    const { unmount } = render(<MoleculeViewer smiles="C" />);

    // Wait for the molecule to be rendered (and thus deleted in finally block)
    await waitFor(() => {
        expect(mockGetMol).toHaveBeenCalled();
    });

    // Wait for the finally block to execute which calls delete
    await waitFor(() => {
        expect(mockDelete).toHaveBeenCalledTimes(1);
    });

    // Unmount the component
    unmount();

    // Check if delete was called again (it shouldn't be)
    expect(mockDelete).toHaveBeenCalledTimes(1);
  });
});
