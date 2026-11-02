/** Content addressing: compute `sha256:<hex>` for a file in the browser. */
export async function sha256File(file: Blob): Promise<string> {
  const buf = await file.arrayBuffer();
  const hash = await crypto.subtle.digest('SHA-256', buf);
  const hex = [...new Uint8Array(hash)].map((b) => b.toString(16).padStart(2, '0')).join('');
  return `sha256:${hex}`;
}
