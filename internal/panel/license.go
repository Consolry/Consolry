package panel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// licencePublicKey checks licence keys. Only consolry.com holds the matching private key,
// so a key that passes this check was made there after a payment.
var licencePublicKey, _ = base64.StdEncoding.DecodeString("x5IHWN2n9oH/lobLgyBI2C96tdP9AW/fzZoK4NP7HrI=")

// licenceRefreshURL gives a fresh key for a subscription that is still paid up.
var licenceRefreshURL = "https://www.consolry.com/api/license"

// Licence is what a licence key says.
type Licence struct {
	Subscription string `json:"sub"`
	Plan         string `json:"plan"`
	// Nodes is how many nodes a Host licence covers; 0 means no limit.
	Nodes   int   `json:"nodes"`
	Expires int64 `json:"exp"`
}

var errBadLicence = errors.New("that is not a valid Consolry licence key")

// readLicence checks a key's signature and returns what it says.
func readLicence(key string) (Licence, error) {
	key = strings.TrimSpace(key)
	body, found := strings.CutPrefix(key, "CONSOLRY-")
	payload64, signature64, split := strings.Cut(body, ".")
	if !found || !split {
		return Licence{}, errBadLicence
	}
	payload, err1 := base64.RawURLEncoding.DecodeString(payload64)
	signature, err2 := base64.RawURLEncoding.DecodeString(signature64)
	if err1 != nil || err2 != nil || len(licencePublicKey) != ed25519.PublicKeySize || !ed25519.Verify(licencePublicKey, payload, signature) {
		return Licence{}, errBadLicence
	}
	var licence Licence
	if err := json.Unmarshal(payload, &licence); err != nil || licence.Plan == "" {
		return Licence{}, errBadLicence
	}
	return licence, nil
}

// Plan is the paid plan this panel has: "plus", "pro" or "host", or "community" when there is none.
func (a *App) Plan() string {
	licence, err := readLicence(a.store.Setting("licence", ""))
	if err != nil || time.Now().Unix() > licence.Expires {
		return "community"
	}
	return licence.Plan
}

// refreshLicence asks consolry.com for a new key, which carries the next renewal date.
func (a *App) refreshLicence(ctx context.Context) error {
	current := a.store.Setting("licence", "")
	if current == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"key": current})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, licenceRefreshURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := updateClient.Do(req)
	if err != nil {
		return errors.New("consolry.com could not be reached")
	}
	defer res.Body.Close()
	var reply struct {
		Key   string `json:"key"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&reply)
	if res.StatusCode != http.StatusOK {
		if reply.Error != "" {
			return errors.New(reply.Error)
		}
		return errors.New("consolry.com answered " + res.Status)
	}
	if _, err := readLicence(reply.Key); err != nil {
		return err
	}
	return a.store.SetSetting("licence", reply.Key)
}

// StartLicenceRenewal refreshes the licence key once a day while the panel runs.
func (a *App) StartLicenceRenewal(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			attempt, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := a.refreshLicence(attempt); err != nil {
				log.Printf("licence not renewed: %v", err)
			}
			cancel()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (a *App) licenceView() map[string]any {
	view := map[string]any{"plan": a.Plan(), "hasKey": false}
	if licence, err := readLicence(a.store.Setting("licence", "")); err == nil {
		view["hasKey"], view["expires"], view["nodes"], view["licensedPlan"] = true, licence.Expires, licence.Nodes, licence.Plan
	}
	return view
}

func (a *App) handleLicence(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.licenceView())
}

func (a *App) handleUpdateLicence(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Key string `json:"key"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	key := strings.TrimSpace(input.Key)
	if key != "" {
		licence, err := readLicence(key)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error()+". Copy the whole key, starting with CONSOLRY-")
			return
		}
		if time.Now().Unix() > licence.Expires {
			writeError(w, http.StatusBadRequest, "that key has run out. Open the panel's Licence page to renew it, or check your subscription")
			return
		}
	}
	if err := a.store.SetSetting("licence", key); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if key != "" {
		// Pick up any renewal straight away; a failure here is not a problem yet.
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		_ = a.refreshLicence(ctx)
	}
	writeJSON(w, http.StatusOK, a.licenceView())
}
