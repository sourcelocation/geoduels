import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Flame } from "lucide-react";
import { useAuthState } from "../../../auth/components/AuthProvider";
import { nominate } from "../../../admin/lib/curation-client";
import { SectionCard } from "../../../../components/ui/compositions";
import { Button, ButtonLink } from "../../../../components/ui/button";
import type { CustomMap } from "../../../maps/lib/maps-client";
export function MapModeratorOperations({map}:{map:CustomMap}) {
 const auth=useAuthState(); const client=useQueryClient();
 const mutation=useMutation({mutationFn:()=>nominate(auth.accessToken,map.id),onSuccess:()=>client.invalidateQueries({queryKey:["motw"]})});
 if(!auth.isModerator)return null;
 const eligible=map.status==="ready"&&map.visibility==="public"&&!!map.ownerUserId;
 return <SectionCard className="rounded-2xl p-5"><h3 className="text-heading-sm font-strong">Moderator Map Operations</h3><div className="mt-3 flex flex-wrap gap-3">
 <Button disabled={!eligible||mutation.isPending||mutation.isSuccess} onClick={()=>mutation.mutate()}><Flame size={16}/>{mutation.isSuccess?"Nominated":"Nominate for Map of the Week"}</Button>
 <ButtonLink href="/admin/moderator/map-of-the-week" variant="secondary">View nominations</ButtonLink></div>
 {!eligible?<p className="mt-2 text-content-secondary">Only ready, public community maps can be nominated.</p>:null}
 {mutation.error?<p role="alert" className="mt-2 text-status-danger">{mutation.error.message}</p>:null}</SectionCard>;
}
