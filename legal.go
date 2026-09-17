package roles

import "strings"

// IsLegalDirectory reports whether one directory name confers licence context.
// It accepts a single name, independent of the repository path contract.
func IsLegalDirectory(name string) bool {
	for _, r := range foldedDirectories {
		if r.Role == Legal && strings.EqualFold(name, r.Pattern) {
			return true
		}
	}
	return false
}

// IsLegalDirectoryBytes is the raw Git-name form of IsLegalDirectory.
func IsLegalDirectoryBytes(name []byte) bool {
	for _, r := range foldedDirectories {
		if r.Role == Legal && equalFoldBytesString(name, r.Pattern) {
			return true
		}
	}
	return false
}

// LegalFileName reports licence and notice matches for one filename.
// These predicates use the same corpus as Classify and allocate no evidence.
func LegalFileName(name string) (license, notice bool) {
	for _, r := range foldedPrefixes {
		if r.Role != Legal || !prefixMatches(name, r.Pattern) {
			continue
		}
		license = license || r.Subtype == "license"
		notice = notice || r.Subtype == "notice"
	}
	return license, notice
}

// LegalFileNameBytes is the raw Git-name form of LegalFileName.
func LegalFileNameBytes(name []byte) (license, notice bool) {
	for _, r := range foldedPrefixes {
		if r.Role != Legal || !prefixMatchesBytes(name, r.Pattern) {
			continue
		}
		license = license || r.Subtype == "license"
		notice = notice || r.Subtype == "notice"
	}
	return license, notice
}

func prefixMatches(name, prefix string) bool {
	if len(name) < len(prefix) {
		return false
	}
	if len(name) > len(prefix) {
		switch name[len(prefix)] {
		case '.', '-', '_':
		default:
			return false
		}
	}
	return strings.EqualFold(name[:len(prefix)], prefix)
}
