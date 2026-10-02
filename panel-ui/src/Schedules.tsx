import { useCallback, useEffect, useState, type FormEvent } from "react";
import { api, formatTime, type NewSchedule, type Schedule, type ScheduleAction, type ScheduleMode, type ServerInfo } from "./api";
import { Empty, ErrorNote, useAction } from "./ui";

const actions: { id: ScheduleAction; label: string }[] = [
  { id: "restart", label: "Restart the server" },
  { id: "backup", label: "Take a backup" },
  { id: "command", label: "Run a console command" },
  { id: "start", label: "Start the server" },
  { id: "stop", label: "Stop the server" },
];
const days = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const intervals = [30, 60, 120, 180, 240, 360, 480, 720, 1440];

function every(minutes: number) {
  if (minutes < 60) return `every ${minutes} minutes`;
  const hours = minutes / 60;
  return hours === 1 ? "every hour" : `every ${hours} hours`;
}

/** A schedule in a sentence, such as "Restart the server every day at 04:00". */
function describe(schedule: Pick<Schedule, "action" | "command" | "mode" | "minutes" | "at" | "weekday">) {
  const what = schedule.action === "command" ? `Run "${schedule.command}"` : actions.find((item) => item.id === schedule.action)!.label;
  const when =
    schedule.mode === "interval" ? every(schedule.minutes) : schedule.mode === "daily" ? `every day at ${schedule.at}` : `every ${days[schedule.weekday]} at ${schedule.at}`;
  return `${what} ${when}`;
}

export default function Schedules({ server }: { server: ServerInfo }) {
  const [list, setList] = useState<Schedule[] | null>(null);
  const [draft, setDraft] = useState<NewSchedule>({ action: "restart", command: "", mode: "daily", minutes: 360, at: "04:00", weekday: 1 });
  const { error, busy, run, setError } = useAction();

  const load = useCallback(() => {
    api.schedules(server.id).then(setList, (problem) => setError((problem as Error).message));
  }, [server.id, setError]);
  useEffect(load, [load]);

  function add(event: FormEvent) {
    event.preventDefault();
    run("add", () => api.createSchedule(server.id, draft), load);
  }

  return (
    <section className="schedules">
      <ErrorNote message={error} />

      {list === null ? null : list.length === 0 ? (
        <Empty title="No schedules yet">A nightly restart and a daily backup are a good start.</Empty>
      ) : (
        <ul className="rows boxed">
          {list.map((schedule) => (
            <li key={schedule.id} className={schedule.enabled ? undefined : "off"}>
              <span className="grow">
                <strong>{describe(schedule)}</strong>
                <small>
                  {schedule.enabled ? `Next: ${formatTime(schedule.nextRun)}` : "Paused"}
                  {schedule.lastRun > 0 && ` · Last: ${formatTime(schedule.lastRun)}, ${schedule.lastResult}`}
                </small>
              </span>
              <button className="small" disabled={busy !== ""} onClick={() => run(`run ${schedule.id}`, () => api.runSchedule(schedule.id), load)}>
                {busy === `run ${schedule.id}` ? "Running…" : "Run now"}
              </button>
              <button className="small" onClick={() => run("toggle", () => api.setScheduleEnabled(schedule.id, !schedule.enabled), load)}>
                {schedule.enabled ? "Pause" : "Resume"}
              </button>
              <button className="small" onClick={() => run("delete", () => api.deleteSchedule(schedule.id), load)}>
                Delete
              </button>
            </li>
          ))}
        </ul>
      )}

      <form className="card form" onSubmit={add}>
        <h2>Add a schedule</h2>
        <label>
          What should happen
          <select value={draft.action} onChange={(event) => setDraft({ ...draft, action: event.target.value as ScheduleAction })}>
            {actions.map((item) => (
              <option key={item.id} value={item.id}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
        {draft.action === "command" && (
          <label>
            Command
            <input
              className="mono"
              value={draft.command}
              onChange={(event) => setDraft({ ...draft, command: event.target.value })}
              placeholder="say The server restarts in 5 minutes"
              required
              spellCheck={false}
            />
            <small>Typed into the console exactly as written. It only works while the server is running.</small>
          </label>
        )}
        <div className="pair">
          <label>
            How often
            <select value={draft.mode} onChange={(event) => setDraft({ ...draft, mode: event.target.value as ScheduleMode })}>
              <option value="daily">Every day</option>
              <option value="weekly">Every week</option>
              <option value="interval">Every few hours</option>
            </select>
          </label>
          {draft.mode === "interval" ? (
            <label>
              Interval
              <select value={draft.minutes} onChange={(event) => setDraft({ ...draft, minutes: Number(event.target.value) })}>
                {intervals.map((minutes) => (
                  <option key={minutes} value={minutes}>
                    {every(minutes)}
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <label>
              At
              <input type="time" value={draft.at} onChange={(event) => setDraft({ ...draft, at: event.target.value })} required />
            </label>
          )}
        </div>
        {draft.mode === "weekly" && (
          <label>
            Day
            <select value={draft.weekday} onChange={(event) => setDraft({ ...draft, weekday: Number(event.target.value) })}>
              {days.map((day, index) => (
                <option key={day} value={index}>
                  {day}
                </option>
              ))}
            </select>
          </label>
        )}
        <p className="dim">
          {describe(draft)}. Times use this machine's clock. Schedules only run while Consolry is running
          {draft.action === "backup" && ", and the newest 7 scheduled backups are kept"}.
        </p>
        <button className="primary" disabled={busy === "add"}>
          Add schedule
        </button>
      </form>
    </section>
  );
}
