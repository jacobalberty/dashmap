import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { APIError } from '../api'

// We test the envelope-unwrap behaviour by mocking global fetch.
// The api functions import from '../api' but use location.origin — stub it too.
Object.defineProperty(globalThis, 'location', {
  value: { origin: 'http://localhost:5173' },
  writable: true,
})

function mockFetch(body: unknown, ok = true, status = 200) {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok,
      status,
      json: async () => body,
    }),
  )
}

beforeEach(() => {
  vi.resetAllMocks()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('APIError', () => {
  it('carries code and message', () => {
    const e = new APIError('not_found', 'clip not found')
    expect(e.code).toBe('not_found')
    expect(e.message).toBe('clip not found')
    expect(e).toBeInstanceOf(Error)
  })
})

describe('API envelope unwrap', () => {
  it('returns data on success', async () => {
    mockFetch({ data: [{ id: 1 }], meta: null, error: null })
    const { getCars } = await import('../api')
    const cars = await getCars()
    expect(cars).toEqual([{ id: 1 }])
  })

  it('throws APIError on envelope error', async () => {
    mockFetch({ data: null, meta: null, error: { code: 'bad_request', message: 'oops' } })
    const { getCars } = await import('../api')
    await expect(getCars()).rejects.toThrow(APIError)
    await expect(getCars()).rejects.toMatchObject({ code: 'bad_request' })
  })
})

describe('getClips', () => {
  it('passes limit and offset as query params', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      json: async () => ({ data: [], meta: { limit: 50, offset: 50, has_more: false }, error: null }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const { getClips } = await import('../api')
    await getClips({ from: '2026-05-01T00:00:00Z', to: '2026-05-02T00:00:00Z', limit: 50, offset: 50 })
    const url: string = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('limit=50')
    expect(url).toContain('offset=50')
  })
})

describe('getCalendar', () => {
  it('passes tz param', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      json: async () => ({ data: [], meta: null, error: null }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const { getCalendar } = await import('../api')
    await getCalendar({ tz: 'America/Los_Angeles' })
    const url: string = fetchMock.mock.calls[0][0] as string
    expect(url).toContain('tz=America%2FLos_Angeles')
  })
})
