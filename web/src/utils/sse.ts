import { API_BASE_URL } from '@/constants'

export class SSERequestError extends Error {
  status: number
  data: unknown

  constructor(status: number, message: string, data: unknown) {
    super(message)
    this.name = 'SSERequestError'
    this.status = status
    this.data = data
  }
}

function parseSSE<T>(reader: ReadableStreamDefaultReader<Uint8Array>, onProgress: (p: T) => void): Promise<void> {
  const decoder = new TextDecoder()
  let buffer = ''
  const read = async (): Promise<void> => {
    const { done, value } = await reader.read()
    if (done) return
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n\n')
    buffer = lines.pop() || ''
    for (const line of lines) {
      if (line.startsWith('data: ')) {
        try {
          onProgress(JSON.parse(line.slice(6)))
        } catch {
          // ignore SSE parse errors
        }
      }
    }
    return read()
  }
  return read()
}

async function createSSEStream(url: string, body: unknown): Promise<ReadableStreamDefaultReader<Uint8Array>> {
  const response = await fetch(`${API_BASE_URL}${url}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream'
    },
    body: JSON.stringify(body)
  })
  if (!response.ok) {
    let data: unknown = null
    try {
      data = await response.json()
    } catch {
      // keep the structured status even when the body is not JSON
    }
    throw new SSERequestError(response.status, `HTTP ${response.status}: ${response.statusText}`, data)
  }
  return response.body?.getReader() || Promise.reject(new Error('No response body'))
}

export async function sseRequest<T>(url: string, body: unknown, onProgress: (p: T) => void): Promise<void> {
  const reader = await createSSEStream(url, body)
  return parseSSE<T>(reader, onProgress)
}
