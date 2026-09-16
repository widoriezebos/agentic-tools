package spend

import (
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

type AttributionWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	Zone string    `json:"zone"`
	Day  string    `json:"day"`
}
type KindAttribution struct {
	Kind   string  `json:"kind"`
	Tokens float64 `json:"tokens"`
}
type ModelAttribution struct {
	Model         string  `json:"model"`
	Input         float64 `json:"input"`
	CacheCreation float64 `json:"cacheCreation"`
	CacheRead     float64 `json:"cacheRead"`
	Output        float64 `json:"output"`
	Reasoning     float64 `json:"reasoning"`
	Total         float64 `json:"total"`
}
type Attribution struct {
	Scope      string             `json:"scope"`
	Window     AttributionWindow  `json:"window"`
	ByKind     []KindAttribution  `json:"byKind"`
	ByModel    []ModelAttribution `json:"byModel"`
	OutOfScope int                `json:"outOfScope"`
	Unknown    int                `json:"unknown"`
}
type attributionCall struct {
	request transcriptRequest
	file    transcriptFile
}

func buildAttribution(now time.Time, zone string, jobs readerJobs, calls map[string]attributionCall) Attribution {
	location, _ := time.LoadLocation(zone)
	local := now.In(location)
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	window := AttributionWindow{From: from, To: from.AddDate(0, 0, 1), Zone: zone, Day: from.Format("2006-01-02")}
	result := Attribution{Scope: readerScopeLabel(readerRegistry), Window: window, ByKind: []KindAttribution{}, ByModel: []ModelAttribution{}}
	readers := map[string]reader{}
	for _, registered := range readerRegistry {
		if registered.inScope {
			readers[registered.name] = registered
		}
	}
	for _, job := range jobs.records {
		stamp, stampErr := parseTime(job.startedAt)
		if stampErr != nil || stamp.Before(window.From) || !stamp.Before(window.To) {
			continue
		}
		registered, inScope := readers[job.runtime]
		if !inScope {
			result.OutOfScope++
		} else if registered.capability == readerCapabilityNone {
			result.Unknown++
		}
	}
	kinds := map[string]float64{}
	models := map[string]*ModelAttribution{}
	for _, call := range calls {
		stamp, stampErr := parseTime(call.request.timestamp)
		if stampErr != nil {
			continue
		}
		kind := "main"
		if call.file.delegate {
			kind = "delegate"
		} else if owner := jobs.owner(call.file.session, call.request.runtime, stamp); owner != nil && owner.role != "steward-continuation" {
			kind, stamp = "engine", owner.start
		}
		if stamp.Before(window.From) || !stamp.Before(window.To) {
			continue
		}
		total := call.request.classes.total()
		kinds[kind] += total
		model := config.CanonicalModel(call.request.model)
		row := models[model]
		if row == nil {
			row = &ModelAttribution{Model: model}
			models[model] = row
		}
		row.add(call.request.classes)
	}
	for kind, tokens := range kinds {
		result.ByKind = append(result.ByKind, KindAttribution{Kind: kind, Tokens: tokens})
	}
	for _, row := range models {
		result.ByModel = append(result.ByModel, *row)
	}
	sort.Slice(result.ByKind, func(i, j int) bool { return result.ByKind[i].Kind < result.ByKind[j].Kind })
	sort.Slice(result.ByModel, func(i, j int) bool { return result.ByModel[i].Model < result.ByModel[j].Model })
	return result
}

type ownedJob struct {
	role  string
	start time.Time
	id    string
}

func (jobs readerJobs) owner(session, runtime string, stamp time.Time) *ownedJob {
	var owners []ownedJob
	for _, job := range jobs.bySession[session] {
		start, err := parseTime(job.startedAt)
		if err == nil && job.runtime == runtime {
			owners = append(owners, ownedJob{job.role, start, job.id})
		}
	}
	if len(owners) == 0 {
		return nil
	}
	sort.Slice(owners, func(i, j int) bool {
		if owners[i].start.Equal(owners[j].start) {
			return owners[i].id < owners[j].id
		}
		return owners[i].start.Before(owners[j].start)
	})
	chosen := &owners[0]
	for index := range owners {
		if !stamp.Before(owners[index].start) {
			chosen = &owners[index]
		}
	}
	return chosen
}

func (classes rawTokenClasses) total() float64 {
	return classes.Input + classes.CacheCreation + classes.CacheRead + classes.Output + classes.Reasoning
}
func (row *ModelAttribution) add(classes rawTokenClasses) {
	row.Input += classes.Input
	row.CacheCreation += classes.CacheCreation
	row.CacheRead += classes.CacheRead
	row.Output += classes.Output
	row.Reasoning += classes.Reasoning
	row.Total += classes.total()
}
