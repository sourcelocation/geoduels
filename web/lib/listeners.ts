export type Listener = () => void;

/** Who wants to hear about a store's changes. A store holds one of these rather than extending anything. */
export class Listeners {
  private readonly listeners = new Set<Listener>();

  subscribe = (listener: Listener) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  emit() {
    for (const listener of this.listeners) listener();
  }
}
