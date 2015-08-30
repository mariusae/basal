package dex // import "basal.io/x/dex"

//go:generate stringer -type=Dir

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"time"
)

const (
	applicationId = "d89443d2-327c-4a6f-89e5-496bbb0317db"
	agent         = "Dexcom Share/3.0.2.11 CFNetwork/711.2.23 Darwin/14.0.0"
	loginUrl      = "https://share1.dexcom.com/ShareWebServices/Services/General/LoginPublisherAccountByName"
	queryUrl      = "https://share1.dexcom.com/ShareWebServices/Services/Publisher/ReadPublisherLatestGlucoseValues"
)

// The type of blood glucose trend (direction).
type Dir int

// Blood glucose trends, as defined by Dexcom.
const (
	None          Dir = iota
	DoubleUp          // ⇈
	SingleUp          // ↑
	FortyFiveUp       // ⇗
	Flat              // →
	FortyFiveDown     // ⇘
	SingleDown        // ↓
	DoubleDown        // ⇊
	NotComputable
	RateOutOfRange
)

func (d Dir) Arrow() string {
	switch d {
	case None:
		return ""
	case DoubleUp:
		return "⇈"
	case SingleUp:
		return "↑"
	case FortyFiveUp:
		return "⇗"
	case Flat:
		return "→"
	case FortyFiveDown:
		return "⇘"
	case SingleDown:
		return "↓"
	case DoubleDown:
		return "⇊"
	default:
		return "?"
	}
}

func (d Dir) Emoji() string {
	switch d {
	case None:
		return ""
	case DoubleUp:
		return "⏫"
	case SingleUp:
		return "⬆️"
	case FortyFiveUp:
		return "↗️"
	case Flat:
		return "➡️"
	case FortyFiveDown:
		return "↘️"
	case SingleDown:
		return "⬇️"
	case DoubleDown:
		return "⏬"
	default:
		return "?"
	}
}

var numToDir = map[int]Dir{
	0: None,
	1: DoubleUp,
	2: SingleUp,
	3: FortyFiveUp,
	4: Flat,
	5: FortyFiveDown,
	6: SingleDown,
	7: DoubleDown,
	8: NotComputable,
	9: RateOutOfRange,
}

var client = http.Client{Transport: &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}}

var datePat = regexp.MustCompile(".*\\(([^)]+)\\).*")

type Session struct {
	token string
	path  string
	user  string
	pass  string
}

type Entry struct {
	Time  time.Time // The walltime of the entry.
	Value int       // The current blood glucose level in mg/dL
	Dir   Dir       // The direction of blood glucose trending.
	Raw   string    // The raw JSON entry in string form.
}

type savedSession struct {
	Token string `json:"token"`
}

func restore(path string) *Session {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	r := bufio.NewReader(file)
	d := json.NewDecoder(r)

	var saved savedSession
	if err := d.Decode(&saved); err != nil {
		return nil
	}

	return &Session{token: saved.Token, path: path}
}

func (s *Session) save() error {
	file, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	defer w.Flush()

	enc := json.NewEncoder(w)
	if err := enc.Encode(savedSession{Token: s.token}); err != nil {
		return err
	}

	return nil
}

// Begin a new session with the given Dexcom username and password.
// Dial will save and restore session tokens in file $HOME/.dex.$user.
func Dial(user, pass string) (*Session, error) {
	path := os.ExpandEnv("$HOME/.dex.") + user
	s := restore(path)
	if s != nil {
		//		log.Printf("restored saved session from %v\n", path)
		s.user = user
		s.pass = pass
		return s, nil
	}

	s = &Session{path: path, user: user, pass: pass}
	if err := s.login(); err != nil {
		return nil, err
	}

	if err := s.save(); err != nil {
		log.Printf("ailed to save session: %v\n", err)
	}

	return s, nil
}

func (s *Session) refresh() error {
	if err := s.login(); err != nil {
		return err
	}
	if err := s.save(); err != nil {
		log.Printf("ailed to save session: %v\n", err)
	}
	return nil
}

type entryJson struct {
	WT    string `json:"WT"`
	Trend int    `json:"Trend"`
	Value int    `json:"Value"`
}

// Retrieve entries since time begin. This is best effort. The underlying
// data may not be available from Dexcom, nor is it guaranteed to be
// complete.
func (s *Session) Tail(howlong time.Duration) ([]Entry, error) {
	var resp *http.Response

	for {
		minutes := howlong.Minutes()
		count := int(minutes) / 5
		params := url.Values{
			"sessionID": {s.token},
			"minutes":   {fmt.Sprintf("%.0f", minutes)},
			"maxCount":  {fmt.Sprintf("%d", count)}}

		req, err := http.NewRequest("POST", queryUrl+"?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		addHeaders(req)
		req.Header.Add("content-length", "0") // necessary?

		tries := 0

		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 400 {
			break
		}

		// Assume token is expired.
		// log.Printf("refreshing token\n")
		if err := s.refresh(); err != nil {
			return nil, err
		}

		// Just back off, and wait for eventual success?
		if tries++; tries == 5 {
			return nil, errors.New(
				fmt.Sprintf("Giving up after %d tries", tries))
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// var pp bytes.Buffer
	// json.Indent(&pp, body, "", "	")
	// log.Printf("body \"%s\"", string(pp.Bytes()))

	var ejs []entryJson
	if err := json.Unmarshal(body, &ejs); err != nil {
		return nil, errors.New(
			fmt.Sprintf("Failed to unmarshal \"%v\": %v", string(body), err))
	}

	entries := make([]Entry, len(ejs))

	for i, ej := range ejs {
		matches := datePat.FindStringSubmatch(ej.WT)
		if matches == nil || len(matches) != 2 {
			return nil, errors.New(fmt.Sprintf("No match for date in %v", ej.WT))
		}

		msecs, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, err
		}

		j := len(entries) - i - 1
		entries[j].Value = ej.Value
		entries[j].Time = time.Unix(msecs/1000, 0)
		entries[j].Dir = numToDir[ej.Trend]
	}

	return entries, nil
}

// Stream entries as they become available. They are written
// to channel out; the channel is closed on error.
func (s *Session) Stream(begin time.Time, out chan<- Entry) {
	// TODO: report skew
	// TODO: base eta on "now" time instead of begin (?),
	// or compute skew based on the difference between
	// this time and wall time?
	//
	// Higher backoff value?

	defer close(out)

	eta := time.Now()
	penalty := 0 * time.Second
	total := 0 * time.Second

	for {
		now := time.Now()
		if eta.After(now) {
			wait := eta.Sub(now)
			time.Sleep(wait)
		}
		time.Sleep(penalty)
		total += penalty

		if penalty < 10*time.Second {
			penalty += time.Second
		}

		// We extend our duration a little bit to give some wiggle
		// room for uneven sampling.
		dur := time.Since(begin) + 5*time.Minute
		ents, err := s.Tail(dur)
		if err != nil {
			log.Printf("Failed to retrieve data\n")
			return
		}

		var newest *Entry
		for i := range ents {
			if ents[i].Time.After(begin) {
				out <- ents[i]
				newest = &ents[i]
			}
		}

		if newest != nil {
			// Dexcom samples every five minutes. Of course some may be
			// missed because devices are offline, or other failures.
			log.Printf("Sampled with penalty %v\n", total)
			begin = newest.Time
			eta = begin.Add(5 * time.Minute)
			penalty = 0 * time.Second
			total = 0 * time.Second
		}
	}
}

func addHeaders(req *http.Request) {
	req.Header.Add("user-agent", agent)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("accept", "application/json")
}

type loginBody struct {
	User          string `json:"accountName"`
	Password      string `json:"password"`
	ApplicationId string `json:"applicationId"`
}

func (s *Session) login() error {
	body := loginBody{
		User:          s.user,
		Password:      s.pass,
		ApplicationId: applicationId}
	bodyJson, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", loginUrl, bytes.NewReader(bodyJson))
	if err != nil {
		return err
	}
	addHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, &s.token)
}
