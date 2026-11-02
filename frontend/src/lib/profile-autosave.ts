// Pending drafts belong to a profile; editing another profile must not discard them.
export class ProfileAutosave {
  private pending = new Map<string, { timer: ReturnType<typeof setTimeout>; save: () => Promise<unknown> }>()
  private versions = new Map<string, number>()

  version(profile: string) { return this.versions.get(profile) ?? 0 }

  cancel(profile: string) {
    const pending = this.pending.get(profile)
    if (pending) clearTimeout(pending.timer)
    this.pending.delete(profile)
    this.versions.set(profile, this.version(profile) + 1)
  }

  schedule(profile: string, save: () => Promise<unknown>) {
    this.cancel(profile)
    const timer = setTimeout(() => { void this.flush(profile) }, 450)
    this.pending.set(profile, { timer, save })
  }

  flush(profile: string) {
    const pending = this.pending.get(profile)
    if (!pending) return Promise.resolve()
    clearTimeout(pending.timer)
    this.pending.delete(profile)
    return pending.save()
  }

  flushAll() { return Promise.all([...this.pending.keys()].map(profile => this.flush(profile))) }
}

export class RequestScope {
  private generation = 0
  begin() { return ++this.generation }
  invalidate() { this.generation++ }
  current(ticket: number) { return ticket === this.generation }
}
