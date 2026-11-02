import { afterEach, describe, expect, it, vi } from 'vitest'
import { ProfileAutosave, RequestScope } from './profile-autosave'

afterEach(() => vi.useRealTimers())
describe('profile autosave', () => {
  it('keeps pending edits of different profiles and coalesces edits of one profile', async () => {
    vi.useFakeTimers()
    const queue = new ProfileAutosave(), save = vi.fn().mockResolvedValue(undefined)
    queue.schedule('A', () => save('A', 1))
    queue.schedule('B', () => save('B', 2))
    queue.schedule('B', () => save('B', 3))
    await vi.advanceTimersByTimeAsync(450)
    expect(save.mock.calls).toEqual([['A', 1], ['B', 3]])
  })
  it('flushes pending drafts on navigation and invalidates old acknowledgments', async () => {
    vi.useFakeTimers()
    const queue = new ProfileAutosave(), save = vi.fn().mockResolvedValue(undefined)
    queue.schedule('A', () => save())
    const version = queue.version('A')
    await queue.flushAll()
    queue.schedule('A', () => save())
    expect(queue.version('A')).not.toBe(version)
    queue.cancel('A')
    await vi.runAllTimersAsync()
    expect(save).toHaveBeenCalledTimes(1)
  })
  it('rejects stale results after selection changes or a newer request', () => {
    const scope = new RequestScope(), first = scope.begin()
    scope.invalidate()
    expect(scope.current(first)).toBe(false)
    const second = scope.begin()
    expect(scope.current(second)).toBe(true)
    scope.begin()
    expect(scope.current(second)).toBe(false)
  })
})
