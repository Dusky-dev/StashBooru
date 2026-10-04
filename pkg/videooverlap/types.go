// Package videooverlap indexes time-stamped visual samples, then reviews pairs.
// It never modifies catalogue relationships, active media or restore files.
package videooverlap

import (
	"fmt"
	"math"
	"time"
)

const Algorithm = "pts-phash-dhash-rgb-bars-v1"
const MaxFrames = 3600

type Config struct {
	Backend       string  `json:"backend"`
	SampleSeconds float64 `json:"sampleSeconds"`
	AudioDigest   bool    `json:"audioDigest"`
}

func DefaultConfig() Config { return Config{Backend: "auto", SampleSeconds: 1} }

func (c Config) Validate() error {
	if c.Backend != "auto" && c.Backend != "local" && c.Backend != "remote" {
		return fmt.Errorf("backend must be auto, local or remote")
	}
	if math.IsNaN(c.SampleSeconds) || math.IsInf(c.SampleSeconds, 0) || c.SampleSeconds < .5 || c.SampleSeconds > 10 {
		return fmt.Errorf("sample interval must be between 0.5 and 10 seconds")
	}
	return nil
}

type MatchOptions struct {
	MaxDistance    int     `json:"maxDistance"`
	MinimumSeconds float64 `json:"minimumSeconds"`
	MaxCandidates  int     `json:"maxCandidates"`
}

func DefaultMatchOptions() MatchOptions {
	return MatchOptions{MaxDistance: 8, MinimumSeconds: 4, MaxCandidates: 200}
}

func (o MatchOptions) Validate() error {
	if o.MaxDistance < 0 || o.MaxDistance > 8 {
		return fmt.Errorf("pHash distance must be between 0 and 8")
	}
	if math.IsNaN(o.MinimumSeconds) || math.IsInf(o.MinimumSeconds, 0) || o.MinimumSeconds < 2 || o.MinimumSeconds > 120 {
		return fmt.Errorf("minimum segment must be between 2 and 120 seconds")
	}
	if o.MaxCandidates < 1 || o.MaxCandidates > 1000 {
		return fmt.Errorf("candidate limit must be between 1 and 1000")
	}
	return nil
}

// Source binds a signature to the native primary file and its measured identity.
type Source struct {
	SceneID     int    `json:"sceneID"`
	FileID      int64  `json:"fileID"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ModTime     int64  `json:"modTime"`
	Fingerprint string `json:"fingerprint"`
}

type Media struct {
	Duration       float64  `json:"duration"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Codec          string   `json:"codec"`
	BitRate        int64    `json:"bitRate"`
	Rotation       int      `json:"rotation"`
	AudioTracks    int      `json:"audioTracks"`
	SubtitleTracks int      `json:"subtitleTracks"`
	AudioCodecs    []string `json:"audioCodecs"`
	AudioSHA256    string   `json:"audioSHA256,omitempty"`
}

type Frame struct {
	Time   float64    `json:"time"`
	Hash   uint64     `json:"hash"`
	DHash  uint64     `json:"dhash"`
	RGB    [3]float64 `json:"rgb"`
	Weight float64    `json:"weight"`
}

type Signature struct {
	Source    Source    `json:"source"`
	SHA256    string    `json:"sha256"`
	Algorithm string    `json:"algorithm"`
	Decoder   string    `json:"decoder"`
	Config    Config    `json:"config"`
	Step      float64   `json:"step"`
	Media     Media     `json:"media"`
	Frames    []Frame   `json:"frames"`
	IndexedAt time.Time `json:"indexedAt"`
}

func (s Signature) Current(source Source, c Config, decoder string) bool {
	return s.Source == source && s.Algorithm == Algorithm && s.Decoder == decoder && s.Config.SampleSeconds == c.SampleSeconds && s.Config.AudioDigest == c.AudioDigest
}

type Range struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type Interval struct {
	A                 Range   `json:"a"`
	B                 Range   `json:"b"`
	Offset            float64 `json:"offset"`
	Samples           int     `json:"samples"`
	DistinctFrames    int     `json:"distinctFrames"`
	MeanDistance      float64 `json:"meanDistance"`
	MeanDHashDistance float64 `json:"meanDHashDistance"`
	Support           float64 `json:"support"`
	GapsA             []Range `json:"gapsA"`
	GapsB             []Range `json:"gapsB"`
}

type Match struct {
	A          int        `json:"a"`
	B          int        `json:"b"`
	Class      string     `json:"class"`
	Intervals  []Interval `json:"intervals"`
	CoverageA  float64    `json:"coverageA"`
	CoverageB  float64    `json:"coverageB"`
	UnmatchedA []Range    `json:"unmatchedA"`
	UnmatchedB []Range    `json:"unmatchedB"`
	Tolerance  float64    `json:"tolerance"`
	Audio      string     `json:"audio"`
	Evidence   string     `json:"evidence"`
	Confidence string     `json:"confidence"`
	Limited    bool       `json:"limited"`
}

type CandidateReport struct {
	IDs                []int `json:"ids"`
	Total              int   `json:"total"`
	OmittedCommonBands int   `json:"omittedCommonBands"`
	Limited            bool  `json:"limited"`
}

type Review struct {
	Reference        int             `json:"reference"`
	Options          MatchOptions    `json:"options"`
	Candidates       CandidateReport `json:"candidates"`
	Matches          []Match         `json:"matches"`
	Skipped          int             `json:"skipped"`
	Errors           []string        `json:"errors"`
	AlignmentLimited int             `json:"alignmentLimited"`
}
