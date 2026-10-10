/** The unseen notifications, oldest first; the live connection keeps them current. */
export function unseenNotificationsKey(userId: string) {
  return ["notifications", userId] as const;
}
