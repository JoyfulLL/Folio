import { KnownFileType } from '@/core/file.ts';

import { HEIC_IMAGE_EXTENSIONS, HEIC_IMAGE_MIME_TYPES } from '@/consts/file.ts';

import { isObject, isString } from '@/lib/common.ts';
import logger from '@/lib/logger.ts';

// This is a locale key as well as a readable fallback. It is shown through te()/tt(),
// so the caller always has a non-empty, translated message instead of an Event object.
export const HEIC_CONVERSION_ERROR_MESSAGE: string = 'Unable to convert HEIC image';

// ISO BMFF ftyp brands used by HEIC/HEIF containers (including the generic mif1/msf1).
const HEIC_FTYP_BRANDS: readonly string[] = [
    'heic', 'heix', 'hevc', 'hevx',
    'heim', 'heis', 'hevm', 'hevs',
    'mif1', 'msf1'
];

// Only used for the intermediate HEIC -> JPEG decode. The user-selected transaction
// picture quality is still applied afterwards by compressJpgImageByQuality().
const HEIC_DECODE_JPEG_QUALITY: number = 0.95;

// Some browsers neither load nor reject an unsupported blob URL, so the native
// decode attempt must not be able to leave the upload spinner stuck forever.
const NATIVE_DECODE_TIMEOUT_MS: number = 3000;

function readAscii(bytes: Uint8Array): string {
    let result = '';

    for (const byte of bytes) {
        result += String.fromCharCode(byte);
    }

    return result;
}

function hasHeicFileExtension(fileName?: string | null): boolean {
    if (!fileName) {
        return false;
    }

    const lowerCaseFileName = fileName.toLowerCase();
    return HEIC_IMAGE_EXTENSIONS.some(extension => lowerCaseFileName.endsWith(`.${extension}`));
}

function hasHeicMimeType(mimeType?: string | null): boolean {
    if (!mimeType) {
        return false;
    }

    return HEIC_IMAGE_MIME_TYPES.includes(mimeType.toLowerCase());
}

async function hasHeicFileSignature(blob: Blob): Promise<boolean> {
    const header = new Uint8Array(await blob.slice(0, 16).arrayBuffer());

    if (header.length < 12 || readAscii(header.slice(4, 8)) !== 'ftyp') {
        return false;
    }

    return HEIC_FTYP_BRANDS.includes(readAscii(header.slice(8, 12)).toLowerCase());
}

/**
 * Detects HEIC/HEIF by file signature first, so a mislabeled file is still handled.
 * Extension and MIME type are only a fallback for truncated or unreadable headers.
 */
export async function isHeicImage(blob: Blob, fileName?: string | null): Promise<boolean> {
    try {
        if (await hasHeicFileSignature(blob)) {
            return true;
        }
    } catch (error) {
        logger.warn('failed to read the image file signature', error);
    }

    return hasHeicFileExtension(fileName) || hasHeicMimeType(blob.type);
}

function decodeImageByBrowser(blob: Blob): Promise<HTMLImageElement> {
    return new Promise((resolve, reject) => {
        const objectUrl = URL.createObjectURL(blob);
        const image = new Image();
        let timer: ReturnType<typeof setTimeout> | null = null;
        let settled = false;

        const settle = (callback: () => void): void => {
            if (settled) {
                return;
            }

            settled = true;

            if (timer) {
                clearTimeout(timer);
            }

            URL.revokeObjectURL(objectUrl);
            callback();
        };

        timer = setTimeout(() => {
            settle(() => reject(new Error('the browser did not finish decoding the image')));
        }, NATIVE_DECODE_TIMEOUT_MS);

        image.onload = () => {
            settle(() => {
                if (!image.naturalWidth || !image.naturalHeight) {
                    reject(new Error('the browser returned an empty image'));
                    return;
                }

                resolve(image);
            });
        };

        image.onerror = () => {
            settle(() => reject(new Error('the browser cannot decode this image')));
        };

        image.src = objectUrl;
    });
}

function drawImageToJpeg(image: HTMLImageElement, quality: number): Promise<Blob> {
    return new Promise((resolve, reject) => {
        const canvas = document.createElement('canvas');
        const context = canvas.getContext('2d');

        if (!context) {
            reject(new Error('failed to get canvas context'));
            return;
        }

        canvas.width = image.naturalWidth;
        canvas.height = image.naturalHeight;
        context.drawImage(image, 0, 0);

        canvas.toBlob(output => {
            if (output && output.size > 0) {
                resolve(output);
            } else {
                reject(new Error('failed to encode the decoded image as JPEG'));
            }
        }, KnownFileType.JPG.contentType, quality);
    });
}

/**
 * Safari/iOS can decode HEIC natively. Using that path avoids downloading the
 * decoder chunk entirely and keeps the original color profile handling of the OS.
 */
async function convertHeicByBrowser(blob: Blob, quality: number): Promise<Blob | null> {
    if (typeof document === 'undefined' || typeof Image === 'undefined' || typeof URL.createObjectURL !== 'function') {
        return null;
    }

    try {
        const image = await decodeImageByBrowser(blob);
        return await drawImageToJpeg(image, quality);
    } catch {
        return null;
    }
}

/**
 * Decoder adapter - the ONLY place that knows which HEIC decoder is used.
 *
 * heic-to is dynamically imported from its CSP-safe build (libheif compiled to
 * JavaScript with USE_UNSAFE_EVAL=0, no eval and no external wasm asset). It is
 * licensed LGPL-3.0-or-later, so it is kept unmodified in a separate lazy chunk,
 * never in the main entry. The distribution carries the GPL-3.0/LGPL-3.0 texts,
 * the prominent notice and the replacement instructions in
 * public/licenses/NOTICE.txt; the About dialog lists the component.
 *
 * A build made with FOLIO_HEIC_DECODER=none aliases heic-to/csp to
 * heic_decoder_stub.ts, so this adapter still compiles but ships zero LGPL code.
 * Swapping in another decoder only requires changing this one function.
 */
async function convertHeicByDecoder(blob: Blob, quality: number): Promise<Blob> {
    const { heicTo } = await import('heic-to/csp');
    const converted = await heicTo({
        blob: blob,
        type: 'image/jpeg',
        quality: quality
    });

    if (!converted || converted.size < 1) {
        throw new Error('the HEIC decoder returned an empty image');
    }

    return converted;
}

/** Converts a HEIC/HEIF blob into a JPEG blob. Throws an error with a displayable message. */
export async function convertHeicToJpeg(blob: Blob, quality: number = HEIC_DECODE_JPEG_QUALITY): Promise<Blob> {
    const nativeResult = await convertHeicByBrowser(blob, quality);

    if (nativeResult) {
        return nativeResult;
    }

    try {
        return await convertHeicByDecoder(blob, quality);
    } catch (error) {
        logger.warn('failed to convert HEIC image to JPEG', error);
        throw new Error(HEIC_CONVERSION_ERROR_MESSAGE);
    }
}

/**
 * Public, reusable entry point for every image upload flow.
 * Non-HEIC blobs are returned untouched, so the existing JPG/JPEG/PNG/GIF/WebP
 * pipeline is not changed. The returned blob is always safe to pass to
 * compressJpgImageByQuality() and to upload as a .jpg file.
 */
export async function normalizeImageToJpeg(blob: Blob, fileName?: string | null): Promise<Blob> {
    if (!(await isHeicImage(blob, fileName))) {
        return blob;
    }

    return convertHeicToJpeg(blob);
}

/** Turns any thrown value into a non-empty message suitable for showError()/showToast(). */
export function getImageUploadErrorMessage(error: unknown, fallbackMessage: string): string {
    if (isString(error) && error) {
        return error;
    }

    if (isObject(error) && 'message' in error) {
        const message = error.message;

        if (isString(message) && message) {
            return message;
        }
    }

    return fallbackMessage;
}
