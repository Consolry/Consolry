package panel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Schedule is one repeating job for a server.
type Schedule struct {
	ID       int64  `json:"id"`
	ServerID string `json:"serverId"`
	// Action is "start", "stop", "restart", "backup" or "command".
	Action  string `json:"action"`
	Command string `json:"command"`
	// Mode is "interval" (every Minutes), "daily" (at At) or "weekly" (on Weekday at At).
	Mode       string `json:"mode"`
	Minutes    int    `json:"minutes"`
	At         string `json:"at"`      // "HH:MM", in the panel machine's local time
	Weekday    int    `json:"weekday"` // 0 is Sunday
	Enabled    bool   `json:"enabled"`
	CreatedAt  int64  `json:"createdAt"`
	LastRun    int64  `json:"lastRun"`
	LastResult string `json:"lastResult"`
	NextRun    int64  `json:"nextRun"`
}

var clockTime = regexp.MustCompile(`^([01][0-9]|2[0-3]):([0-5][0-9])$`)

func (s *Schedule) validate() error {
	switch s.Action {
	case "start", "stop", "restart", "backup":
		s.Command = ""
	case "command":
		s.Command = strings.TrimSpace(s.Command)
		if s.Command == "" || strings.ContainsAny(s.Command, "\r\n") {
			return errors.New("enter one console command on a single line")
		}
	default:
		return errors.New("choose what the schedule should do")
	}
	switch s.Mode {
	case "interval":
		if s.Minutes < 5 || s.Minutes > 7*24*60 {
			return errors.New("the interval must be between 5 minutes and 7 days")
		}
	case "daily", "weekly":
		if !clockTime.MatchString(s.At) {
			return errors.New("enter the time as HH:MM")
		}
		if s.Weekday < 0 || s.Weekday > 6 {
			return errors.New("choose a day of the week")
		}
	default:
		return errors.New("choose when the schedule should run")
	}
	return nil
}

// catchUp is how late a clock-time schedule may still run. A daily 4am restart that was
// missed because the panel was off should not fire at 3pm when the panel comes back.
const catchUp = 5 * time.Minute

// next returns when the schedule should run next, given the current time.
func (s Schedule) next(now time.Time) time.Time {
	last := time.Unix(s.CreatedAt, 0)
	if s.LastRun > s.CreatedAt {
		last = time.Unix(s.LastRun, 0)
	}
	if s.Mode == "interval" {
		return last.Add(time.Duration(s.Minutes) * time.Minute)
	}

	after := last
	if floor := now.Add(-catchUp); floor.After(after) {
		after = floor
	}
	hour, _ := strconv.Atoi(s.At[:2])
	minute, _ := strconv.Atoi(s.At[3:])
	after = after.In(now.Location())
	candidate := time.Date(after.Year(), after.Month(), after.Day(), hour, minute, 0, 0, now.Location())
	for !candidate.After(after) || (s.Mode == "weekly" && int(candidate.Weekday()) != s.Weekday) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

// --- storage ---

const scheduleColumns = `id, server_id, action, command, mode, minutes, at, weekday, enabled, created_at, last_run, last_result`

func scanSchedule(row interface{ Scan(...any) error }) (Schedule, error) {
	var s Schedule
	err := row.Scan(&s.ID, &s.ServerID, &s.Action, &s.Command, &s.Mode, &s.Minutes, &s.At, &s.Weekday, &s.Enabled, &s.CreatedAt, &s.LastRun, &s.LastResult)
	return s, err
}

func (s *Store) Schedules(serverID string) ([]Schedule, error) {
	query, args := `SELECT `+scheduleColumns+` FROM schedules`, []any{}
	if serverID != "" {
		query += ` WHERE server_id = ?`
		args = append(args, serverID)
	}
	rows, err := s.db.Query(query+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Schedule{}
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, schedule)
	}
	return list, rows.Err()
}

func (s *Store) Schedule(id int64) (Schedule, error) {
	return scanSchedule(s.db.QueryRow(`SELECT `+scheduleColumns+` FROM schedules WHERE id = ?`, id))
}

func (s *Store) CreateSchedule(schedule Schedule) (Schedule, error) {
	schedule.CreatedAt, schedule.Enabled = time.Now().Unix(), true
	result, err := s.db.Exec(`INSERT INTO schedules (server_id, action, command, mode, minutes, at, weekday, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		schedule.ServerID, schedule.Action, schedule.Command, schedule.Mode, schedule.Minutes, schedule.At, schedule.Weekday, schedule.CreatedAt)
	if err != nil {
		return Schedule{}, err
	}
	schedule.ID, err = result.LastInsertId()
	return schedule, err
}

func (s *Store) SetScheduleEnabled(id int64, enabled bool) error {
	// Re-enabling counts from now, so a schedule that was off for a week does not fire at once.
	_, err := s.db.Exec(`UPDATE schedules SET enabled = ?, created_at = ? WHERE id = ?`, enabled, time.Now().Unix(), id)
	return err
}

func (s *Store) DeleteSchedule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM schedules WHERE id = ?`, id)
	return err
}

func (s *Store) markScheduleRun(id int64, at time.Time, result string) {
	_, _ = s.db.Exec(`UPDATE schedules SET last_run = ?, last_result = ? WHERE id = ?`, at.Unix(), result, id)
}

// --- running ---

// serverState asks the node for one server's live state.
func serverState(ctx context.Context, node Node, id string) (daemonSpec, error) {
	var specs []daemonSpec
	if err := node.call(ctx, http.MethodGet, "/servers", nil, &specs); err != nil {
		return daemonSpec{}, err
	}
	for _, spec := range specs {
		if spec.ID == id {
			return spec, nil
		}
	}
	return daemonSpec{}, errors.New("the node no longer has this server")
}

func power(ctx context.Context, node Node, id, action string) error {
	return node.call(ctx, http.MethodPost, "/servers/"+id+"/"+action, nil, nil)
}

// restart stops a running server, waits for it to finish shutting down, and starts it again.
func restart(ctx context.Context, node Node, id string) error {
	spec, err := serverState(ctx, node, id)
	if err != nil {
		return err
	}
	if spec.State == "running" || spec.State == "starting" {
		if err := power(ctx, node, id, "stop"); err != nil {
			return err
		}
	}
	deadline := time.Now().Add(2 * time.Minute)
	for {
		spec, err = serverState(ctx, node, id)
		if err != nil {
			return err
		}
		if spec.State == "offline" || spec.State == "crashed" {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("the server did not stop within two minutes")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return power(ctx, node, id, "start")
}

// scheduledBackupsKept is how many scheduled backups stay on disk per server.
const scheduledBackupsKept = 7

// runSchedule carries out a schedule's action and returns a one-line result for the list.
func (a *App) runSchedule(ctx context.Context, schedule Schedule) string {
	row, err := a.store.Server(schedule.ServerID)
	if err != nil {
		return "Failed: the server no longer exists"
	}
	node, err := a.store.Node(row.NodeID)
	if err != nil {
		return "Failed: the server's node no longer exists"
	}
	switch schedule.Action {
	case "start", "stop":
		err = power(ctx, node, row.ID, schedule.Action)
	case "restart":
		err = restart(ctx, node, row.ID)
	case "backup":
		err = node.slow(ctx, http.MethodPost, fmt.Sprintf("/servers/%s/backups?kind=scheduled&keep=%d", row.ID, scheduledBackupsKept), nil, nil)
	case "command":
		err = node.call(ctx, http.MethodPost, "/servers/"+row.ID+"/command", map[string]string{"command": schedule.Command}, nil)
	}
	if err != nil {
		return "Failed: " + err.Error()
	}
	return "Done"
}

// StartScheduler runs due schedules until ctx ends.
func (a *App) StartScheduler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				schedules, err := a.store.Schedules("")
				if err != nil {
					log.Printf("scheduler: %v", err)
					continue
				}
				for _, schedule := range schedules {
					if !schedule.Enabled || schedule.next(now).After(now) {
						continue
					}
					// Record the run before starting it, so a slow job is never started twice.
					a.store.markScheduleRun(schedule.ID, now, "Running…")
					go func(schedule Schedule) {
						result := a.runSchedule(ctx, schedule)
						a.store.markScheduleRun(schedule.ID, now, result)
						a.store.Log(schedule.ServerID, "schedule", "Scheduled "+schedule.Action+": "+result)
					}(schedule)
				}
			}
		}
	}()
}

// --- API ---

func (a *App) handleSchedules(w http.ResponseWriter, r *http.Request) {
	row, _, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	list, err := a.store.Schedules(row.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := time.Now()
	for i := range list {
		if list[i].Enabled {
			list[i].NextRun = list[i].next(now).Unix()
		}
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *App) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var input Schedule
	if !readJSON(w, r, &input) {
		return
	}
	row, _, ok := a.serverNode(w, r)
	if !ok {
		return
	}
	input.ServerID = row.ID
	if err := input.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := a.store.CreateSchedule(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a.log(r, row.ID, "Added a schedule ("+created.Action+")")
	writeJSON(w, http.StatusCreated, created)
}

// schedule loads the schedule named in the request path, answering 404 itself if it is missing.
func (a *App) schedule(w http.ResponseWriter, r *http.Request) (Schedule, bool) {
	id, _ := strconv.ParseInt(r.PathValue("sid"), 10, 64)
	schedule, err := a.store.Schedule(id)
	if errors.Is(err, sql.ErrNoRows) || id == 0 {
		writeError(w, http.StatusNotFound, "schedule not found")
		return Schedule{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return Schedule{}, false
	}
	return schedule, true
}

func (a *App) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	schedule, ok := a.schedule(w, r)
	if !ok {
		return
	}
	if err := a.store.SetScheduleEnabled(schedule.ID, input.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	schedule, ok := a.schedule(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteSchedule(schedule.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleRunSchedule(w http.ResponseWriter, r *http.Request) {
	schedule, ok := a.schedule(w, r)
	if !ok {
		return
	}
	now := time.Now()
	result := a.runSchedule(r.Context(), schedule)
	a.store.markScheduleRun(schedule.ID, now, result)
	a.log(r, schedule.ServerID, "Ran the "+schedule.Action+" schedule by hand: "+result)
	writeJSON(w, http.StatusOK, map[string]string{"result": result})
}
