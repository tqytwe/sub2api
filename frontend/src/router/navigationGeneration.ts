/**
 * Tracks navigation ownership for asynchronous router guards. A guard may
 * finish loading lazy resources after a newer navigation has already started;
 * only the latest guard may then write document-level state or resolve a route.
 */
export interface NavigationGeneration {
  begin(): number
  isCurrent(generation: number): boolean
}

export function createNavigationGeneration(): NavigationGeneration {
  let current = 0

  return {
    begin: () => ++current,
    isCurrent: (generation) => generation === current,
  }
}
