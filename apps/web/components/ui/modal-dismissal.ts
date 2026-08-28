type ModalDismissRequest = () => Promise<void>;

const mountedModals = new Set<ModalDismissRequest>();

export function registerModalDismissRequest(request: ModalDismissRequest) {
  mountedModals.add(request);
  return () => mountedModals.delete(request);
}

export function hasMountedModals() {
  return mountedModals.size > 0;
}

/** Requests an exit from every mounted modal and waits for their animations. */
export async function dismissAllModals() {
  await Promise.allSettled(Array.from(mountedModals, (request) => request()));
}
