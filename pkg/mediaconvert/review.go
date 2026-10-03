package mediaconvert

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// These are server-owned policy, never options sent to an encoder/legacy worker.
type SavingsThresholds struct {
	MinimumSavedBytes   int64   `json:"minimumSavedBytes"`
	MinimumSavedPercent float64 `json:"minimumSavedPercent"`
}

func (s SavingsThresholds) Validate() error {
	if s.MinimumSavedBytes < 0 || math.IsNaN(s.MinimumSavedPercent) || math.IsInf(s.MinimumSavedPercent, 0) || s.MinimumSavedPercent < 0 || s.MinimumSavedPercent > 100 {
		return fmt.Errorf("savings minimums must be nonnegative bytes and a percentage from 0 to 100")
	}
	return nil
}

func (s SavingsThresholds) Reason(before, after int64) string {
	if before <= 0 || after <= 0 {
		return "input and verified output must have positive sizes; source kept"
	}
	saved := before - after
	if saved <= 0 {
		return "output was not smaller; source kept"
	}
	if saved < s.MinimumSavedBytes {
		return "saved bytes below minimum; source kept"
	}
	if 100*float64(saved)/float64(before) < s.MinimumSavedPercent {
		return "saved percentage below minimum; source kept"
	}
	return ""
}

func conversionSkipReason(before, after int64, options Options, savings *SavingsThresholds) string {
	if savings != nil {
		return savings.Reason(before, after)
	}
	if !options.AllowLarger && after >= before {
		return "output was not smaller; source kept"
	}
	return ""
}

func (c Config) ValidateReview() error {
	if err := c.Savings.Validate(); err != nil {
		return err
	}
	if c.TrialCacheLimitBytes < 0 || c.TrialTTLHours < 0 || c.TrialTTLHours > 168 {
		return fmt.Errorf("trial cache must be nonnegative and expiry between 1 and 168 hours")
	}
	return nil
}

func (c Config) TrialTTL() time.Duration {
	hours := c.TrialTTLHours
	if hours == 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

type SampleCandidate struct {
	Key              string `json:"key"`
	Input            string `json:"input"`
	Animated         bool   `json:"animated"`
	AnimationUnknown bool   `json:"animationUnknown"`
	Size             int64  `json:"size"`
}

func (c SampleCandidate) Stratum() string {
	bucket := "large (>16 MiB)"
	if c.Size <= 1024*1024 {
		bucket = "small (≤1 MiB)"
	} else if c.Size <= 16*1024*1024 {
		bucket = "medium (1–16 MiB)"
	}
	status := "still"
	if c.Animated {
		status = "animated"
	}
	if c.AnimationUnknown {
		status = "uninspected animation"
	}
	return fmt.Sprintf("%s / %s / %s", c.Input, status, bucket)
}

// SelectEstimateSample is deterministic and stratified by format, animation and
// size. It selects size quantiles within each stratum, never a first-page prefix.
func SelectEstimateSample(candidates []SampleCandidate, limit int) []SampleCandidate {
	if limit <= 0 {
		return nil
	}
	groups := map[string][]SampleCandidate{}
	for _, c := range candidates {
		groups[c.Stratum()] = append(groups[c.Stratum()], c)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for key, group := range groups {
		sort.Slice(group, func(i, j int) bool {
			if group[i].Size != group[j].Size {
				return group[i].Size < group[j].Size
			}
			return group[i].Key < group[j].Key
		})
		groups[key] = group
	}
	ret := []SampleCandidate{}
	seen := map[string]bool{}
	for _, fraction := range []float64{0.5, 0, 1} {
		for _, key := range keys {
			group := groups[key]
			c := group[int(float64(len(group)-1)*fraction)]
			if seen[c.Key] {
				continue
			}
			ret = append(ret, c)
			seen[c.Key] = true
			if len(ret) >= limit {
				return ret
			}
		}
	}
	return ret
}

type EstimateMeasurement struct {
	Candidate   SampleCandidate
	OutputBytes int64
	Eligible    bool
	Error       string
}

type EstimateStratum struct {
	Name                string `json:"name"`
	Count               int    `json:"count"`
	SourceBytes         int64  `json:"sourceBytes"`
	Sampled             int    `json:"sampled"`
	Failed              int    `json:"failed"`
	SampleSourceBytes   int64  `json:"sampleSourceBytes"`
	SampleOutputBytes   int64  `json:"sampleOutputBytes"`
	EstimatedSavedBytes int64  `json:"estimatedSavedBytes"`
	ObservedLowBytes    int64  `json:"observedLowBytes"`
	ObservedHighBytes   int64  `json:"observedHighBytes"`
}

type Estimate struct {
	Total               int               `json:"total"`
	Sampled             int               `json:"sampled"`
	Covered             bool              `json:"covered"`
	EstimatedSavedBytes int64             `json:"estimatedSavedBytes"`
	ObservedLowBytes    int64             `json:"observedLowBytes"`
	ObservedHighBytes   int64             `json:"observedHighBytes"`
	Strata              []EstimateStratum `json:"strata"`
}

// Failed/uncovered strata prevent an overall extrapolation. The observed range
// is a sample envelope, not a statistical confidence interval or a guarantee.
func SummarizeEstimate(candidates []SampleCandidate, measurements []EstimateMeasurement) Estimate {
	ret := Estimate{Total: len(candidates), Covered: true, Strata: []EstimateStratum{}}
	groups := map[string]*EstimateStratum{}
	ranges := map[string][2]float64{}
	for _, c := range candidates {
		key := c.Stratum()
		if groups[key] == nil {
			groups[key] = &EstimateStratum{Name: key}
		}
		groups[key].Count++
		groups[key].SourceBytes += c.Size
	}
	for _, m := range measurements {
		v := groups[m.Candidate.Stratum()]
		if v == nil {
			continue
		}
		ret.Sampled++
		if m.Error != "" || m.Candidate.Size <= 0 || m.OutputBytes <= 0 {
			v.Failed++
			continue
		}
		fraction := 0.0
		if m.Eligible {
			fraction = float64(m.Candidate.Size-m.OutputBytes) / float64(m.Candidate.Size)
		}
		rangeValue := ranges[v.Name]
		if v.Sampled == 0 {
			rangeValue = [2]float64{fraction, fraction}
		} else {
			rangeValue[0] = min(rangeValue[0], fraction)
			rangeValue[1] = max(rangeValue[1], fraction)
		}
		ranges[v.Name] = rangeValue
		v.Sampled++
		v.SampleSourceBytes += m.Candidate.Size
		// Ineligible outputs project zero realized savings, never negative savings.
		if m.Eligible {
			v.SampleOutputBytes += m.OutputBytes
		} else {
			v.SampleOutputBytes += m.Candidate.Size
		}
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		v := groups[key]
		if v.Sampled == 0 || v.Failed > 0 {
			ret.Covered = false
		} else {
			v.EstimatedSavedBytes = int64(float64(v.SourceBytes) * float64(v.SampleSourceBytes-v.SampleOutputBytes) / float64(v.SampleSourceBytes))
			v.ObservedLowBytes = int64(float64(v.SourceBytes) * ranges[key][0])
			v.ObservedHighBytes = int64(float64(v.SourceBytes) * ranges[key][1])
		}
		ret.Strata = append(ret.Strata, *v)
		ret.EstimatedSavedBytes += v.EstimatedSavedBytes
		ret.ObservedLowBytes += v.ObservedLowBytes
		ret.ObservedHighBytes += v.ObservedHighBytes
	}
	if !ret.Covered {
		ret.EstimatedSavedBytes, ret.ObservedLowBytes, ret.ObservedHighBytes = 0, 0, 0
	}
	return ret
}
