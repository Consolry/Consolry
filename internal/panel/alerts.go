package panel

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DinoNaedYT/Consolry/internal/minecraft"
)

// Events a server's alerts can be sent for.
var alertEvents = []string{"crash", "task", "start", "stop"}

// Events for the panel as a whole, which only the admin sets up.
var panelEvents = []string{"update", "node"}

// Alerts is where one server's notices go, or the panel's own when ServerID is empty.
type Alerts struct {
	Discord string   `json:"discord"`
	Email   string   `json:"email"`
	Events  []string `json:"events"`
}

func (s *Store) Alerts(serverID string) Alerts {
	var alerts Alerts
	var events string
	if s.db.QueryRow(`SELECT discord, email, events FROM alerts WHERE server_id = ?`, serverID).Scan(&alerts.Discord, &alerts.Email, &events) != nil {
		// Crashes and failed tasks are worth knowing about from the start.
		if serverID == "" {
			return Alerts{Events: []string{"update", "node"}}
		}
		return Alerts{Events: []string{"crash", "task"}}
	}
	alerts.Events = []string{}
	if events != "" {
		alerts.Events = strings.Split(events, ",")
	}
	return alerts
}

func (s *Store) SetAlerts(serverID string, alerts Alerts) error {
	_, err := s.db.Exec(`
		INSERT INTO alerts (server_id, discord, email, events) VALUES (?, ?, ?, ?)
		ON CONFLICT (server_id) DO UPDATE SET discord = excluded.discord, email = excluded.email, events = excluded.events`,
		serverID, alerts.Discord, alerts.Email, strings.Join(alerts.Events, ","))
	return err
}

func (a Alerts) wants(event string) bool {
	for _, chosen := range a.Events {
		if chosen == event {
			return true
		}
	}
	return false
}

// checkAlerts tidies what was sent in and refuses anything that could point the panel
// at a machine other than Discord's, or at more than a few inboxes.
func checkAlerts(input Alerts, allowed []string) (Alerts, error) {
	clean := Alerts{Discord: strings.TrimSpace(input.Discord), Email: strings.TrimSpace(input.Email), Events: []string{}}
	if clean.Discord != "" {
		parsed, err := url.Parse(clean.Discord)
		if err != nil || parsed.Scheme != "https" || (parsed.Host != "discord.com" && parsed.Host != "discordapp.com" && parsed.Host != "ptb.discord.com" && parsed.Host != "canary.discord.com") ||
			!strings.HasPrefix(parsed.Path, "/api/webhooks/") {
			return Alerts{}, errors.New("paste a Discord webhook address. It starts with https://discord.com/api/webhooks/")
		}
	}
	if clean.Email != "" {
		list, err := mail.ParseAddressList(clean.Email)
		if err != nil || len(list) > 5 {
			return Alerts{}, errors.New("enter up to five email addresses, separated by commas")
		}
		addresses := []string{}
		for _, address := range list {
			addresses = append(addresses, address.Address)
		}
		clean.Email = strings.Join(addresses, ", ")
	}
	for _, event := range allowed {
		for _, chosen := range input.Events {
			if chosen == event {
				clean.Events = append(clean.Events, event)
				break
			}
		}
	}
	return clean, nil
}

// --- sending ---

// SMTP is the mail server the panel sends email alerts through, set by the admin.
type SMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	From     string `json:"from"`
}

func (s *Store) smtp() SMTP {
	var settings SMTP
	_ = json.Unmarshal([]byte(s.Setting("smtp", "{}")), &settings)
	return settings
}

var alertClient = &http.Client{Timeout: 15 * time.Second}

func sendDiscord(ctx context.Context, webhook, text string) error {
	body, _ := json.Marshal(map[string]any{"username": "Consolry", "content": text, "allowed_mentions": map[string]any{"parse": []string{}}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := alertClient.Do(req)
	if err != nil {
		return errors.New("Discord could not be reached")
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("Discord refused the message (%d). Check the webhook address", res.StatusCode)
	}
	return nil
}

func sendEmail(settings SMTP, to, subject, text string) error {
	if settings.Host == "" || settings.From == "" {
		return errors.New("email is not set up yet. The admin sets the mail server on the Alerts page")
	}
	port := settings.Port
	if port == 0 {
		port = 587
	}
	recipients := []string{}
	for _, address := range strings.Split(to, ",") {
		if address = strings.TrimSpace(address); address != "" {
			recipients = append(recipients, address)
		}
	}
	message := "From: Consolry <" + settings.From + ">\r\n" +
		"To: " + strings.Join(recipients, ", ") + "\r\n" +
		"Subject: " + strings.ReplaceAll(subject, "\n", " ") + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n"
	address := net.JoinHostPort(settings.Host, strconv.Itoa(port))
	var auth smtp.Auth
	if settings.Username != "" {
		auth = smtp.PlainAuth("", settings.Username, settings.Password, settings.Host)
	}
	if port != 465 {
		// SendMail switches to an encrypted connection whenever the server offers it.
		return smtp.SendMail(address, auth, settings.From, recipients, []byte(message))
	}
	// Port 465 expects encryption from the first byte.
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", address, &tls.Config{ServerName: settings.Host})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, settings.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(settings.From); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// deliver sends one notice to everywhere the alerts point, and reports what went wrong.
func (a *App) deliver(ctx context.Context, alerts Alerts, subject, text string) error {
	var problems []string
	if alerts.Discord != "" {
		if err := sendDiscord(ctx, alerts.Discord, "**"+subject+"**\n"+text); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if alerts.Email != "" {
		if err := sendEmail(a.store.smtp(), alerts.Email, subject, text); err != nil {
			problems = append(problems, "Email: "+err.Error())
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, ". "))
	}
	return nil
}

// notify sends a server's notice for an event, if its owner asked for that event.
// serverID "" means the panel's own notices.
func (a *App) notify(serverID, event, subject, text string) {
	alerts := a.store.Alerts(serverID)
	if !alerts.wants(event) || (alerts.Discord == "" && alerts.Email == "") {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.deliver(ctx, alerts, subject, text); err != nil {
			log.Printf("alert for %q not sent: %v", serverID, err)
			if serverID != "" {
				a.store.Log(serverID, "consolry", "Could not send an alert: "+err.Error())
			}
		}
	}()
}

// --- watching servers ---

// watcher remembers what it last saw, so it can tell when something changed.
type watcher struct {
	mu          sync.Mutex
	states      map[string]string
	nodeDown    map[int64]time.Time
	nodeWarned  map[int64]bool
	lastUpdate  string
	initialised bool
}

// StartWatching checks every server's state every ten seconds and sends alerts on changes.
// It also tells the admin about new versions of Consolry.
func (a *App) StartWatching(ctx context.Context) {
	a.watch = &watcher{states: map[string]string{}, nodeDown: map[int64]time.Time{}, nodeWarned: map[int64]bool{}}
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		updates := time.NewTicker(6 * time.Hour)
		defer updates.Stop()
		a.watchOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.watchOnce(ctx)
			case <-updates.C:
				a.checkForUpdate(ctx)
			}
		}
	}()
}

func (a *App) checkForUpdate(ctx context.Context) {
	status := a.updates.check(ctx, a.version, false)
	a.watch.mu.Lock()
	seen := a.watch.lastUpdate
	a.watch.lastUpdate = status.Latest
	a.watch.mu.Unlock()
	if status.Available && status.Latest != seen {
		a.notify("", "update", "Consolry "+status.Latest+" is available",
			"You have "+status.Current+". Open the panel and press Update now to install it. "+status.Notes)
	}
}

func (a *App) watchOnce(ctx context.Context) {
	nodes, err := a.store.Nodes()
	if err != nil {
		return
	}
	rows, err := a.store.Servers()
	if err != nil {
		return
	}
	names := map[string]ServerRow{}
	for _, row := range rows {
		names[strconv.FormatInt(row.NodeID, 10)+"/"+row.ID] = row
	}

	a.watch.mu.Lock()
	first := !a.watch.initialised
	a.watch.initialised = true
	a.watch.mu.Unlock()

	for _, node := range nodes {
		check, cancel := context.WithTimeout(ctx, 8*time.Second)
		var specs []daemonSpec
		err := node.call(check, http.MethodGet, "/servers", nil, &specs)
		cancel()

		a.watch.mu.Lock()
		if err != nil {
			// A node is only reported once it has been unreachable for a minute.
			if a.watch.nodeDown[node.ID].IsZero() {
				a.watch.nodeDown[node.ID] = time.Now()
			} else if time.Since(a.watch.nodeDown[node.ID]) > time.Minute && !a.watch.nodeWarned[node.ID] {
				a.watch.nodeWarned[node.ID] = true
				a.notify("", "node", "Machine \""+node.Name+"\" is not answering", "Consolry cannot reach the machine \""+node.Name+"\", so its servers cannot be managed. Check that it is switched on and that Consolry is running on it.")
			}
			a.watch.mu.Unlock()
			continue
		}
		if a.watch.nodeWarned[node.ID] {
			a.notify("", "node", "Machine \""+node.Name+"\" is back", "Consolry can reach \""+node.Name+"\" again.")
		}
		delete(a.watch.nodeDown, node.ID)
		delete(a.watch.nodeWarned, node.ID)

		for _, spec := range specs {
			key := strconv.FormatInt(node.ID, 10) + "/" + spec.ID
			before := a.watch.states[key]
			a.watch.states[key] = spec.State
			row, known := names[key]
			if first || !known || before == spec.State {
				continue
			}
			switch {
			case spec.State == "crashed":
				go a.alertCrash(node, row)
			case spec.State == "running" && before != "running":
				a.notify(row.ID, "start", row.Name+" is running", row.Name+" has started and players can join.")
			case spec.State == "offline" && (before == "running" || before == "stopping"):
				a.notify(row.ID, "stop", row.Name+" stopped", row.Name+" was stopped.")
			}
		}
		a.watch.mu.Unlock()
	}
}

// alertCrash sends a crash notice, with the crash explainer's reading of the log when it has one.
func (a *App) alertCrash(node Node, row ServerRow) {
	text := row.Name + " stopped unexpectedly."
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var lines []string
	if node.call(ctx, http.MethodGet, "/servers/"+row.ID+"/console/history", nil, &lines) == nil {
		if findings := minecraft.Explain(lines); len(findings) > 0 {
			text += "\n\nWhat went wrong: " + findings[0].Title + "\nTo fix it: " + findings[0].Fix
		} else if len(lines) > 0 {
			text += "\n\nLast line of the log: " + lines[len(lines)-1]
		}
	}
	a.notify(row.ID, "crash", row.Name+" crashed", text)
}

// --- the pages that set alerts up ---

func (a *App) handleServerAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.store.Alerts(r.PathValue("id")))
}

func (a *App) handleUpdateServerAlerts(w http.ResponseWriter, r *http.Request) {
	a.saveAlerts(w, r, r.PathValue("id"), alertEvents)
}

func (a *App) handlePanelAlerts(w http.ResponseWriter, r *http.Request) {
	settings := a.store.smtp()
	hasPassword := settings.Password != ""
	settings.Password = ""
	writeJSON(w, http.StatusOK, map[string]any{"alerts": a.store.Alerts(""), "smtp": settings, "smtpPasswordSet": hasPassword})
}

func (a *App) handleUpdatePanelAlerts(w http.ResponseWriter, r *http.Request) {
	a.saveAlerts(w, r, "", panelEvents)
}

func (a *App) saveAlerts(w http.ResponseWriter, r *http.Request, serverID string, allowed []string) {
	var input struct {
		Alerts
		Test bool `json:"test"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	clean, err := checkAlerts(input.Alerts, allowed)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.store.SetAlerts(serverID, clean); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if input.Test {
		if clean.Discord == "" && clean.Email == "" {
			writeError(w, http.StatusBadRequest, "add a Discord webhook or an email address first")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := a.deliver(ctx, clean, "Test alert from Consolry", "Alerts are working. You will get messages like this one when something happens."); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, clean)
}

func (a *App) handleUpdateSMTP(w http.ResponseWriter, r *http.Request) {
	var input SMTP
	if !readJSON(w, r, &input) {
		return
	}
	input.Host, input.From, input.Username = strings.TrimSpace(input.Host), strings.TrimSpace(input.From), strings.TrimSpace(input.Username)
	if input.Host != "" {
		if _, err := mail.ParseAddress(input.From); err != nil {
			writeError(w, http.StatusBadRequest, "enter the address the email should come from")
			return
		}
		if input.Port < 1 || input.Port > 65535 {
			writeError(w, http.StatusBadRequest, "enter the mail server's port, usually 587 or 465")
			return
		}
	}
	// An empty password keeps the one already saved, so it never has to be sent back to the browser.
	if input.Password == "" && input.Host != "" {
		input.Password = a.store.smtp().Password
	}
	data, _ := json.Marshal(input)
	if err := a.store.SetSetting("smtp", string(data)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
