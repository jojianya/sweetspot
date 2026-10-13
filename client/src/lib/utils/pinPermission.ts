/**
 * Whether the viewer may edit or delete the pin: its owner, or an admin or
 * owner role. Shared by the detail panel and the permalink page so the gate
 * cannot drift between the two surfaces.
 */
export function isPinOwner(
  viewer: { id: string; role: string } | null,
  pinUserId: string | null
): boolean {
  return (
    viewer !== null &&
    (viewer.id === pinUserId || viewer.role === "admin" || viewer.role === "owner")
  );
}
