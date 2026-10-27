package catalog

import "strings"

func isPublic(it Item) bool {
	return it.VerificationStatus == "" || it.VerificationStatus == VerificationVerified
}

func publisherID(it Item) string {
	if it.PublisherID != "" {
		return it.PublisherID
	}
	if it.Publisher != nil {
		return it.Publisher.ID
	}
	if it.Creator != nil {
		return it.Creator.ID
	}
	return ""
}

func isOwner(it Item, userID string) bool {
	if userID == "" {
		return false
	}
	return publisherID(it) == userID
}

func canView(it Item, userID string, admin bool) bool {
	if isPublic(it) {
		return true
	}
	if admin {
		return true
	}
	return isOwner(it, userID)
}

func publicItems(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if isPublic(it) {
			out = append(out, it)
		}
	}
	return out
}

func applyViewer(it Item, userID string, admin bool) Item {
	it.Viewer.Owner = isOwner(it, userID)
	it.Viewer.Admin = admin
	if !it.Viewer.Owner && !admin && it.VerificationStatus == VerificationRejected {
		it.RejectionReason = ""
	}
	return it
}

func matchesTypeFilter(it Item, typeSlug string) bool {
	typeSlug = strings.TrimSpace(strings.ToLower(typeSlug))
	if typeSlug == "" || typeSlug == "all" {
		return true
	}
	return it.Type == typeSlug || it.Kind == typeSlug
}
