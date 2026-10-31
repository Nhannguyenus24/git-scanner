package notify

import "github.com/mservice/gitscanner/libs/entity"

// The pieces of the disclosure mail a shared report reuses, so the two say the
// same things about a finding and give the same removal advice.

// GuideStep is one step of the removal guide.
type GuideStep = guideStep

// RemovalGuide is the step-by-step advice for the files actually involved.
func RemovalGuide(repo entity.Repository, findings []entity.Finding) []GuideStep {
	return removalGuide(repo, findings)
}

// FindingLink links to the line at the commit that introduced the finding, or
// is empty when the host is not GitHub or the commit is unknown.
func FindingLink(repo entity.Repository, f entity.Finding) string { return blobURL(repo, f) }

// ProviderName is the finding's provider as a reader would name it.
func ProviderName(f entity.Finding) string { return providerName(f) }

// OrderForReport puts what to rotate first at the top: confirmed live, then by
// severity. The caller's slice is not modified.
func OrderForReport(findings []entity.Finding) []entity.Finding { return sortedBySeverity(findings) }
