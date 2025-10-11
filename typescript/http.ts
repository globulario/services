
import { config } from './config';

/**
 * Shared headers for HTTP helpers
 */
async function sharedHeaders(extra?: HeadersInit): Promise<HeadersInit> {
  const token = config.tokenProvider ? await config.tokenProvider() : '';
  const h: Record<string,string> = {
    'Accept': 'application/json',
    ...(config.application ? { 'application': config.application } : {}),
    ...(config.domain ? { 'domain': config.domain } : {}),
    ...(config.extraHeaders || {}),
  };
  if (token) h['Authorization'] = token.startsWith('Bearer ') ? token : `Bearer ${token}`;
  return { ...h, ...(extra || {}) };
}

function baseUrl(): string {
  const { protocol, domain, ports } = config;
  const port = protocol === 'https' ? ports?.https : ports?.http;
  return `${protocol}://${domain}${port ? `:${port}` : ''}`;
}

/** GET JSON helper */
async function getJSON<T = any>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${baseUrl()}${path}`, {
    method: 'GET',
    headers: await sharedHeaders(init?.headers),
    ...init
  });
  if (!res.ok) throw new Error(`GET ${path} failed: ${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

/** POST JSON helper */
async function postJSON<T = any>(path: string, body: any, init?: RequestInit): Promise<T> {
  const res = await fetch(`${baseUrl()}${path}`, {
    method: 'POST',
    headers: await sharedHeaders({ 'Content-Type': 'application/json', ...(init?.headers || {}) }),
    body: JSON.stringify(body),
    ...init
  });
  if (!res.ok) throw new Error(`POST ${path} failed: ${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

/** Upload a file with form-data; extraFields merged into form */
async function upload(path: string, file: File | Blob, extraFields?: Record<string, string | number | boolean>): Promise<Response> {
  const form = new FormData();
  form.append('file', file);
  if (extraFields) {
    Object.entries(extraFields).forEach(([k, v]) => form.append(k, String(v)));
  }
  const res = await fetch(`${baseUrl()}${path}`, {
    method: 'POST',
    headers: await sharedHeaders(), // don't set Content-Type; browser will set boundary
    body: form
  });
  if (!res.ok) throw new Error(`UPLOAD ${path} failed: ${res.status} ${res.statusText}`);
  return res;
}

/** Download as blob; caller decides how to save */
async function download(path: string, init?: RequestInit): Promise<Blob> {
  const res = await fetch(`${baseUrl()}${path}`, {
    method: 'GET',
    headers: await sharedHeaders(init?.headers),
    ...init
  });
  if (!res.ok) throw new Error(`DOWNLOAD ${path} failed: ${res.status} ${res.statusText}`);
  return res.blob();
}

export const http = { getJSON, postJSON, upload, download };
