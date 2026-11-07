
import { apiFetch, authHeaders, expectJSON, readError } from "../../../lib/http";
export type Nomination = {id: number; mapId: string; name: string; authorName: string; thumbnailKey: string; likes: number; liked: boolean; nominatedAt: string};
export type NominationPage = {items: Nomination[]; total: number; page: number; pageSize: number; closesAt: string};
export type CurationScheduleUpdate = {expectedClosesAt: string; closesAt: string};
export async function reschedulePromotion(token: string, schedule: CurationScheduleUpdate) {
 const response = await apiFetch("/api/staff/motw/schedule", {method: "PUT", headers: {...authHeaders(token), "Content-Type": "application/json"}, body: JSON.stringify(schedule)});
 if (!response.ok) throw new Error(await readError(response, "Could not reschedule promotion"));
}
export async function nominations(token: string, page: number): Promise<NominationPage> {
 return expectJSON(await apiFetch(`/api/staff/motw?page=${page}`, {headers: authHeaders(token)}), "Nominations unavailable");
}
export async function nominate(token: string, mapId: string) {
 const response = await apiFetch(`/api/staff/motw/maps/${encodeURIComponent(mapId)}`, {method: "POST", headers: authHeaders(token)});
 if (!response.ok) throw new Error(await readError(response, "Could not nominate map"));
}
export async function likeNomination(token: string, id: number, liked: boolean) {
 const response = await apiFetch(`/api/staff/motw/nominations/${id}/like`, {method: "PUT", headers: {...authHeaders(token), "Content-Type": "application/json"}, body: JSON.stringify({liked})});
 if (!response.ok) throw new Error(await readError(response, "Could not update like"));
}
