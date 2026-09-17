package roles

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// CorpusVersion identifies classification semantics and evidence ordering.
const CorpusVersion = "2"

//go:embed corpus/rules.json
var corpusData []byte

type rule struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Pattern         string   `json:"pattern"`
	Role            Role     `json:"role"`
	Subtype         string   `json:"subtype,omitempty"`
	Ecosystem       string   `json:"ecosystem,omitempty"`
	Source          string   `json:"source"`
	Extensions      []string `json:"extensions,omitempty"`
	bit             Set
	directorySuffix string
}

var (
	directoryRules    = map[string][]*rule{}
	filenameRules     = map[string][]*rule{}
	foldedStems       = map[int][]*rule{}
	foldedDirectories []*rule
	pathDirectories   = map[string][]*rule{}
	foldedPrefixes    []*rule
	suffixRules       [256][]*rule
)

func init() {
	var rules []rule
	if err := json.Unmarshal(corpusData, &rules); err != nil {
		panic(err)
	}
	seen := map[string]bool{}
	for i := range rules {
		r := &rules[i]
		r.bit = roleBit(r.Role)
		r.directorySuffix = "/" + r.Pattern
		if r.ID == "" || seen[r.ID] || r.bit == 0 || r.Pattern == "" || r.Source == "" {
			panic(fmt.Sprintf("invalid corpus rule %q", r.ID))
		}
		seen[r.ID] = true
		switch r.Kind {
		case "directory":
			directoryRules[r.Pattern] = append(directoryRules[r.Pattern], r)
		case "filename":
			filenameRules[r.Pattern] = append(filenameRules[r.Pattern], r)
		case "stem-fold":
			if len(r.Extensions) == 0 {
				panic(fmt.Sprintf("missing extensions for %q", r.ID))
			}
			for _, ext := range r.Extensions {
				if ext != "" && !strings.HasPrefix(ext, ".") {
					panic(fmt.Sprintf("invalid extension for %q", r.ID))
				}
			}
			foldedStems[len(r.Pattern)] = append(foldedStems[len(r.Pattern)], r)
		case "directory-fold":
			foldedDirectories = append(foldedDirectories, r)
		case "directory-path":
			base := r.Pattern[strings.LastIndexByte(r.Pattern, '/')+1:]
			pathDirectories[base] = append(pathDirectories[base], r)
		case "prefix-fold":
			foldedPrefixes = append(foldedPrefixes, r)
		case "suffix", "suffix-fold":
			last := suffixBucket(r.Pattern[len(r.Pattern)-1])
			suffixRules[last] = append(suffixRules[last], r)
		default:
			panic(fmt.Sprintf("unknown rule kind %q", r.Kind))
		}
	}
}

func suffixBucket(last byte) byte {
	if last >= 'A' && last <= 'Z' {
		return last + ('a' - 'A')
	}
	return last
}
