package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Group preferences are draft UI selections, NOT a description of the live
// Meshing control tree. Unchecking a previously published item never immediately edits ANSYS;
// the verified ACT publisher suppresses owned controls only after explicit confirmation.
type savedNamedChoice struct {
	Scope  []int   `json:"face_ids"`
	Active bool    `json:"active"`
	SizeMM float64 `json:"size_mm"`
}
type savedNamedProfile struct {
	Project    string                      `json:"project"`
	Geometry   string                      `json:"geometry_fingerprint"`
	SavedAt    string                      `json:"saved_at"`
	Selections map[string]savedNamedChoice `json:"selections"`
}
type savedNamedProfiles struct {
	Version  int                          `json:"version"`
	Profiles map[string]savedNamedProfile `json:"profiles"`
}

func namedPrefsKey(project, geometry string) string {
	s := sha256.Sum256([]byte(strings.TrimSpace(project) + "\x00" + strings.TrimSpace(geometry)))
	return hex.EncodeToString(s[:])
}
func usableMM(v float64) bool { return v >= 0.35 && v <= 500 && !math.IsNaN(v) && !math.IsInf(v, 0) }

// Constrain state restoration to existing, unique, nonempty face scopes.
// A changed scope never inherits an old value simply because the name matches.
func makeSavedNamedProfile(s *Snapshot, named []Named, active, paused map[string]float64) (savedNamedProfile, error) {
	if s == nil || strings.TrimSpace(s.Project) == "" || strings.TrimSpace(s.Fingerprint) == "" {
		return savedNamedProfile{}, errors.New("missing current project/geometry; cannot persist group choices")
	}
	p := savedNamedProfile{Project: s.Project, Geometry: s.Fingerprint,
		SavedAt: time.Now().Format(time.RFC3339), Selections: map[string]savedNamedChoice{}}
	for _, n := range named {
		if n.Name == "" || len(n.IDs) == 0 || strings.EqualFold(n.Name, "inlet") || strings.EqualFold(n.Name, "outlet") {
			continue
		}
		mm, enabled := active[n.Name]
		if !enabled {
			mm = paused[n.Name]
		}
		if !usableMM(mm) {
			// Explicitly persist disabled without a manual size. The UI can
			// show a fresh recommendation when the user next enables it.
			mm = 0
		}
		ids := append([]int(nil), n.IDs...)
		sort.Ints(ids)
		p.Selections[n.Name] = savedNamedChoice{Scope: ids, Active: enabled, SizeMM: mm}
	}
	return p, nil
}

func restoreSavedNamedProfile(p savedNamedProfile, s *Snapshot, named []Named) (map[string]float64, map[string]float64, int, error) {
	active := map[string]float64{}
	paused := map[string]float64{}
	if s == nil || p.Project != s.Project || p.Geometry != s.Fingerprint {
		return active, paused, 0, errors.New("saved group choices belong to a different model")
	}
	// Avoid implicitly mapping two different controls onto a single scope.
	oldCounts := map[string]int{}
	newCounts := map[string]int{}
	oldByScope := map[string]string{}
	for n, c := range p.Selections {
		key := namedScopeKey(c.Scope)
		if key != "" {
			oldCounts[key]++
			oldByScope[key] = n
		}
	}
	for _, n := range named {
		if key := namedScopeKey(n.IDs); key != "" {
			newCounts[key]++
		}
	}
	matches := 0
	for _, n := range named {
		if n.Name == "" || len(n.IDs) == 0 || strings.EqualFold(n.Name, "inlet") || strings.EqualFold(n.Name, "outlet") {
			continue
		}
		key := namedScopeKey(n.IDs)
		choice, exists := p.Selections[n.Name]
		if exists {
			if key == "" || namedScopeKey(choice.Scope) != key {
				continue
			}
		} else if key != "" && oldCounts[key] == 1 && newCounts[key] == 1 {
			choice, exists = p.Selections[oldByScope[key]]
		}
		if !exists {
			continue
		}
		matches++
		if choice.Active {
			if usableMM(choice.SizeMM) {
				active[n.Name] = choice.SizeMM
			}
		} else if usableMM(choice.SizeMM) {
			paused[n.Name] = choice.SizeMM
		}
	}
	return active, paused, matches, nil
}

// Storage lives in the configurable AutoMesh data workspace. Never overwrite
// a corrupt/unknown-version settings file: retain it for manual recovery.
func readNamedProfiles(path string) (savedNamedProfiles, error) {
	v := savedNamedProfiles{Version: 1, Profiles: map[string]savedNamedProfile{}}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return v, nil
	}
	if err != nil {
		return v, err
	}
	if info.Size() > 2<<20 {
		return v, fmt.Errorf("group preference file too large: %d bytes", info.Size())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	if err = json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("invalid group preference JSON: %w", err)
	}
	if v.Version != 1 {
		return v, fmt.Errorf("unsupported group preference schema %d", v.Version)
	}
	if v.Profiles == nil {
		v.Profiles = map[string]savedNamedProfile{}
	}
	return v, nil
}
func saveNamedPreferences(path string, p savedNamedProfile) error {
	if p.Project == "" || p.Geometry == "" {
		return errors.New("missing model identity")
	}
	v, err := readNamedProfiles(path)
	if err != nil {
		return err
	}
	key := namedPrefsKey(p.Project, p.Geometry)
	v.Profiles[key] = p
	// Bound growth if the application is used with numerous independent models.
	if len(v.Profiles) > 50 {
		type entry struct{ Key, At string }
		entries := []entry{}
		for k, p := range v.Profiles {
			entries = append(entries, entry{k, p.SavedAt})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].At < entries[j].At })
		for _, e := range entries[:len(entries)-50] {
			delete(v.Profiles, e.Key)
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// Keep a readable backup of the previous valid file, then replace atomically.
	if old, err := os.ReadFile(path); err == nil {
		if e := os.WriteFile(path+".bak", old, 0644); e != nil {
			return e
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeAtomic(path, append(b, '\n'))
}
func loadNamedPreferences(path string, s *Snapshot, named []Named) (map[string]float64, map[string]float64, int, bool, error) {
	v, err := readNamedProfiles(path)
	if err != nil {
		return nil, nil, 0, false, err
	}
	if s == nil {
		return nil, nil, 0, false, nil
	}
	p, found := v.Profiles[namedPrefsKey(s.Project, s.Fingerprint)]
	if !found {
		return nil, nil, 0, false, nil
	}
	active, paused, matched, err := restoreSavedNamedProfile(p, s, named)
	return active, paused, matched, true, err
}
