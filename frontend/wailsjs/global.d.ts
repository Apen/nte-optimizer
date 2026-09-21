declare global {
  interface Window {
    go: Record<string, Record<string, Record<string, (...args: any[]) => Promise<any>>>>
    runtime: { EventsOn: (name: string, callback: (data: any) => void) => () => void }
  }
}

export {}
