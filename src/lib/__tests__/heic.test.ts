import { describe, expect, it, vi } from 'vitest';
import { heicTo } from 'heic-to/csp';

import {
	HEIC_CONVERSION_ERROR_MESSAGE,
	convertHeicToJpeg,
	getImageUploadErrorMessage,
	isHeicImage,
	normalizeImageToJpeg
} from '@/lib/heic.ts';

vi.mock('heic-to/csp', () => ({
	heicTo: vi.fn()
}));

function createImageBlob(bytes: number[], type: string = ''): Blob {
	return new Blob([new Uint8Array(bytes)], { type: type });
}

function createHeicBlob(brand: string, type: string = 'image/heic'): Blob {
	const bytes = [0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0, 0, 0, 0, 0, 0, 0, 0];

	for (let i = 0; i < 4; i++) {
		bytes[8 + i] = brand.charCodeAt(i);
	}

	return createImageBlob(bytes, type);
}

const JPEG_BYTES: number[] = [0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0, 1, 1, 0, 0, 1];

describe('isHeicImage', () => {
	it('should detect the heic ftyp brand', async () => {
		expect(await isHeicImage(createHeicBlob('heic'), 'photo.jpg')).toBe(true);
	});

	it('should detect generic HEIF brands and mixed case', async () => {
		expect(await isHeicImage(createHeicBlob('mif1'), 'photo.bin')).toBe(true);
		expect(await isHeicImage(createHeicBlob('HEIC'), 'photo.bin')).toBe(true);
		expect(await isHeicImage(createHeicBlob('hevc'), 'photo.bin')).toBe(true);
	});

	it('should fall back to the file extension when the header is unreadable', async () => {
		const blob = createImageBlob(JPEG_BYTES, 'image/jpeg');
		expect(await isHeicImage(blob, 'receipt.HEIC')).toBe(true);
		expect(await isHeicImage(blob, 'receipt.heif')).toBe(true);
	});

	it('should fall back to the MIME type when the header is unreadable', async () => {
		expect(await isHeicImage(createImageBlob([1, 2, 3, 4], 'image/heif'), 'photo.jpg')).toBe(true);
	});

	it('should not treat existing formats as HEIC', async () => {
		expect(await isHeicImage(createImageBlob(JPEG_BYTES, 'image/jpeg'), 'photo.jpg')).toBe(false);
		expect(await isHeicImage(createImageBlob([0x89, 0x50, 0x4E, 0x47, 0, 0, 0, 0], 'image/png'), 'photo.png')).toBe(false);
	});
});

describe('normalizeImageToJpeg', () => {
	it('should return the original blob untouched for non-HEIC images', async () => {
		const blob = createImageBlob(JPEG_BYTES, 'image/jpeg');
		expect(await normalizeImageToJpeg(blob, 'photo.jpg')).toBe(blob);
		expect(vi.mocked(heicTo)).not.toHaveBeenCalled();
	});
});

describe('convertHeicToJpeg', () => {
	it('should return the decoder output', async () => {
		const output = createImageBlob(JPEG_BYTES, 'image/jpeg');
		vi.mocked(heicTo).mockResolvedValue(output);

		expect(await convertHeicToJpeg(createHeicBlob('heic'))).toBe(output);
		expect(vi.mocked(heicTo)).toHaveBeenCalledWith({
			blob: expect.any(Blob),
			type: 'image/jpeg',
			quality: 0.95
		});
	});

	it('should reject with a readable message when decoding fails', async () => {
		vi.mocked(heicTo).mockRejectedValue(new Error('libheif internal error'));

		await expect(convertHeicToJpeg(createHeicBlob('heic'))).rejects.toThrow(HEIC_CONVERSION_ERROR_MESSAGE);
	});

	it('should reject with a readable message when the decoder returns an empty blob', async () => {
		vi.mocked(heicTo).mockResolvedValue(new Blob([], { type: 'image/jpeg' }));

		await expect(convertHeicToJpeg(createHeicBlob('heic'))).rejects.toThrow(HEIC_CONVERSION_ERROR_MESSAGE);
	});
});

describe('getImageUploadErrorMessage', () => {
	it('should use the message of an Error', () => {
		expect(getImageUploadErrorMessage(new Error(HEIC_CONVERSION_ERROR_MESSAGE), 'fallback')).toBe(HEIC_CONVERSION_ERROR_MESSAGE);
	});

	it('should use the message of a rejection object', () => {
		expect(getImageUploadErrorMessage({ message: 'Unable to upload transaction picture' }, 'fallback')).toBe('Unable to upload transaction picture');
	});

	it('should use a plain string', () => {
		expect(getImageUploadErrorMessage('Unable to convert HEIC image', 'fallback')).toBe('Unable to convert HEIC image');
	});

	it('should never return an empty message for Event-like or null errors', () => {
		expect(getImageUploadErrorMessage(new Event('error'), 'fallback')).toBe('fallback');
		expect(getImageUploadErrorMessage(null, 'fallback')).toBe('fallback');
		expect(getImageUploadErrorMessage(undefined, 'fallback')).toBe('fallback');
		expect(getImageUploadErrorMessage({ message: '' }, 'fallback')).toBe('fallback');
	});

	it('should fall back for API error responses, which callers pass through to te() instead', () => {
		expect(getImageUploadErrorMessage({ error: { errorCode: 1001 } }, 'fallback')).toBe('fallback');
	});
});
