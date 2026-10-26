package scan

// DetectStats counts what detection did with one scan's input: blobs it never
// looked inside, candidates each filter dropped, and what it reported. It is
// the evidence for tuning — a gate that drops thousands of candidates on real
// repositories is doing work, one that never fires is dead weight, and a jump
// in one after a change says which gate a regression came through.
//
// It is a fixed array indexed by stat rather than a map because the generic
// pass increments it once per high-entropy token, which on a large repository
// is millions of times. A nil *DetectStats counts nothing, so Detect's callers
// that do not want counts pay nothing for them.
type DetectStats struct {
	n [statCount]int
}

type stat int

const (
	statBlobBinary stat = iota
	statBlobDeniedPath

	statNamedCandidate
	statNamedDuplicate
	statNamedKeywordMissing
	statNamedPlaceholder
	statNamedSuppressed
	statNamedLocalURL
	statNamedReported

	statGenericSkippedExampleFile
	statGenericToken
	statGenericDuplicate
	statGenericLowEntropy
	statGenericPlaceholder
	statGenericVariableRef
	statGenericEmbeddedImage
	statGenericURLPath
	statGenericIdentifier
	statGenericTestPath
	statGenericPublicIdentifier
	statGenericPublicKey
	statGenericNoSecretKeyword
	statGenericDevLocal
	statGenericExampleLine
	statGenericJWT
	statGenericReported

	statCount
)

// statNames are the keys the counters are stored and reported under. They are
// persisted, so renaming one splits its history.
var statNames = [statCount]string{
	statBlobBinary:     "blobs_binary",
	statBlobDeniedPath: "blobs_denied_path",

	statNamedCandidate:      "named_candidates",
	statNamedDuplicate:      "named_drop_duplicate",
	statNamedKeywordMissing: "named_drop_keyword_missing",
	statNamedPlaceholder:    "named_drop_placeholder",
	statNamedSuppressed:     "named_drop_suppressed",
	statNamedLocalURL:       "named_drop_local_url",
	statNamedReported:       "named_reported",

	statGenericSkippedExampleFile: "generic_skip_example_file",
	statGenericToken:              "generic_candidates",
	statGenericDuplicate:          "generic_drop_duplicate",
	statGenericLowEntropy:         "generic_drop_low_entropy",
	statGenericPlaceholder:        "generic_drop_placeholder",
	statGenericVariableRef:        "generic_drop_variable_ref",
	statGenericEmbeddedImage:      "generic_drop_embedded_image",
	statGenericURLPath:            "generic_drop_url_path",
	statGenericIdentifier:         "generic_drop_identifier",
	statGenericTestPath:           "generic_drop_test_path",
	statGenericPublicIdentifier:   "generic_drop_public_identifier",
	statGenericPublicKey:          "generic_drop_public_key_name",
	statGenericNoSecretKeyword:    "generic_drop_no_secret_keyword",
	statGenericDevLocal:           "generic_drop_dev_local",
	statGenericExampleLine:        "generic_drop_example_line",
	statGenericJWT:                "generic_drop_jwt",
	statGenericReported:           "generic_reported",
}

func (s *DetectStats) inc(k stat) {
	if s != nil {
		s.n[k]++
	}
}

// Map returns the non-zero counters by name.
func (s *DetectStats) Map() map[string]int {
	out := map[string]int{}
	if s == nil {
		return out
	}
	for i, v := range s.n {
		if v != 0 {
			out[statNames[i]] = v
		}
	}
	return out
}
