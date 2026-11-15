/**
 * Build-time replacement for `heic-to/csp`, used only when the frontend is
 * built with FOLIO_HEIC_DECODER=none.
 *
 * heic-to is LGPL-3.0-or-later. Some deployments want an artifact that ships
 * no LGPL code at all, so Vite aliases the decoder import to this stub in
 * that build (see vite.config.ts). The HEIC file is still detected and the
 * browser's native decoder is still tried first; only the software fallback
 * is unavailable, and the upload shows a normal (non-empty) error instead.
 *
 * Keep the exported name and signature compatible with heicTo() so the
 * adapter in src/lib/heic.ts does not need to know which build it is in.
 * See public/licenses/NOTICE.txt for the full license and replacement notes.
 */
export async function heicTo(_options?: { blob: Blob, type: string, quality: number }): Promise<Blob> {
    throw new Error('HEIC decoder is not included in this build');
}
