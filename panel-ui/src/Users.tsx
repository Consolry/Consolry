import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, permissionList, type Member, type Permission, type ServerInfo } from "./api";
import { ErrorNote, useAction } from "./ui";

function PermissionBoxes({ chosen, onChange, minecraft }: { chosen: Permission[]; onChange: (next: Permission[]) => void; minecraft: boolean }) {
  const shown = permissionList.filter((item) => minecraft || !item.minecraftOnly);
  return (
    <fieldset className="permissions">
      <legend>What they can do</legend>
      {shown.map((item) => (
        <label key={item.id} className="check">
          <input
            type="checkbox"
            checked={chosen.includes(item.id)}
            onChange={(event) => onChange(event.target.checked ? [...chosen, item.id] : chosen.filter((id) => id !== item.id))}
          />
          <span>
            <strong>{item.label}</strong>
            <small>{item.about}</small>
          </span>
        </label>
      ))}
    </fieldset>
  );
}

/** The people a server is shared with. Only its owner sees this tab. */
export default function Users({ server }: { server: ServerInfo }) {
  const [members, setMembers] = useState<Member[] | null>(null);
  const [username, setUsername] = useState("");
  const [chosen, setChosen] = useState<Permission[]>(["console", "power"]);
  const [editing, setEditing] = useState<{ userId: number; permissions: Permission[] } | null>(null);
  const { error, busy, run } = useAction();
  const minecraft = server.kind === "minecraft";

  const load = useCallback(() => api.members(server.id).then(setMembers, () => setMembers([])), [server.id]);
  useEffect(() => {
    load();
  }, [load]);

  function invite(event: FormEvent) {
    event.preventDefault();
    run("invite", () => api.invite(server.id, username.trim(), chosen), () => {
      setUsername("");
      load();
    });
  }

  const labels = (permissions: Permission[]) =>
    permissions.length === 0
      ? "Can only look at the dashboard"
      : permissionList
          .filter((item) => permissions.includes(item.id))
          .map((item) => item.label)
          .join(", ");

  return (
    <>
      <p className="dim lead">
        Share this server with other people. Each person signs in with their own account and can do only what you tick here. Everyone you add can see the
        dashboard.
      </p>
      <ErrorNote message={error} />

      {members && members.length > 0 && (
        <ul className="members">
          {members.map((member) => (
            <li key={member.userId}>
              <div className="member-head">
                <strong>{member.username}</strong>
                <span className="dim grow">{labels(member.permissions)}</span>
                {editing?.userId === member.userId ? (
                  <>
                    <button
                      className="small primary"
                      disabled={busy !== ""}
                      onClick={() =>
                        run("save", () => api.setMemberPermissions(server.id, member.userId, editing.permissions), () => {
                          setEditing(null);
                          load();
                        })
                      }
                    >
                      Save
                    </button>
                    <button className="small" onClick={() => setEditing(null)}>
                      Cancel
                    </button>
                  </>
                ) : (
                  <>
                    <button className="small" onClick={() => setEditing({ userId: member.userId, permissions: member.permissions })}>
                      Change
                    </button>
                    <button className="small danger" disabled={busy !== ""} onClick={() => run("remove", () => api.removeMember(server.id, member.userId), load)}>
                      Remove
                    </button>
                  </>
                )}
              </div>
              {editing?.userId === member.userId && (
                <PermissionBoxes chosen={editing.permissions} onChange={(permissions) => setEditing({ userId: member.userId, permissions })} minecraft={minecraft} />
              )}
            </li>
          ))}
        </ul>
      )}
      {members && members.length === 0 && <p className="dim">This server is not shared with anyone yet.</p>}

      <form className="card form wide" onSubmit={invite}>
        <h2>Invite someone</h2>
        <label>
          Their username
          <input value={username} onChange={(event) => setUsername(event.target.value)} required autoComplete="off" spellCheck={false} />
          <small>They need an account on this panel first. They can make one from the sign-in page with "Create an account".</small>
        </label>
        <PermissionBoxes chosen={chosen} onChange={setChosen} minecraft={minecraft} />
        <button className="primary" disabled={busy !== ""}>
          Invite
        </button>
      </form>
    </>
  );
}
