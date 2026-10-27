package catalog

import "time"

type Pagination struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"perPage"`
	Total      int  `json:"total"`
	TotalPages int  `json:"totalPages"`
	HasNext    bool `json:"hasNext"`
	HasPrev    bool `json:"hasPrev"`
	NextPage   *int `json:"nextPage"`
	PrevPage   *int `json:"prevPage"`
}

type Metric struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Value   int    `json:"value"`
	Display string `json:"display"`
}

type ItemStats struct {
	Saves     int `json:"saves"`
	Likes     int `json:"likes"`
	Forks     int `json:"forks"`
	Pulls     int `json:"pulls"`
	Runs      int `json:"runs"`
	Followers int `json:"followers"`
	Builds    int `json:"builds"`
}

type ViewerState struct {
	Saved bool `json:"saved"`
	Liked bool `json:"liked"`
}

type CreatorRef struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	Verified    bool   `json:"verified"`
}

type CategoryRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type SubItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Mark string `json:"mark,omitempty"`
}

type Item struct {
	ID            string      `json:"id"`
	Slug          string      `json:"slug"`
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	Kind          string      `json:"kind"`
	Mark          string      `json:"mark"`
	Tone          string      `json:"tone"`
	Blurb         string      `json:"blurb"`
	Description   string      `json:"description"`
	Version       string      `json:"version,omitempty"`
	BuildArtwork  string      `json:"buildArtwork"`
	Banners       []string    `json:"banners,omitempty"`
	CategoryID    string      `json:"categoryId"`
	SectionID     string      `json:"sectionId"`
	Category      CategoryRef `json:"category"`
	Stats         ItemStats   `json:"stats"`
	Metric        Metric      `json:"metric"`
	Viewer        ViewerState `json:"viewer"`
	Creator       *CreatorRef `json:"creator,omitempty"`
	Publisher     *Publisher  `json:"publisher,omitempty"`
	PublisherID   string      `json:"-"`
	SourceURL     string      `json:"sourceUrl,omitempty"`
	Tags          []string    `json:"tags"`
	SubItems      []SubItem   `json:"subItems,omitempty"`
	PublishedAt   time.Time   `json:"publishedAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	TrendingScore float64     `json:"trendingScore"`
}

type Category struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SectionID   string `json:"sectionId"`
	ItemCount   int    `json:"itemCount"`
	Href        string `json:"href"`
}

type ItemPage struct {
	Items      []Item     `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type Section struct {
	ID         string     `json:"id"`
	Slug       string     `json:"slug"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle"`
	ItemCount  int        `json:"itemCount"`
	Href       string     `json:"href"`
	Categories []Category `json:"categories,omitempty"`
	Feed       ItemPage   `json:"feed"`
	Trending   *ItemPage  `json:"trending,omitempty"`
}

type Rail struct {
	ID         string     `json:"id"`
	Slug       string     `json:"slug"`
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle"`
	Algorithm  string     `json:"algorithm,omitempty"`
	Href       string     `json:"href"`
	Items      []Item     `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type SearchHints struct {
	Placeholder string        `json:"placeholder"`
	Chips       []CategoryRef `json:"chips"`
}

type Viewer struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	GreetingName string `json:"greetingName"`
	FullName     string `json:"fullName"`
	AvatarURL    string `json:"avatarUrl"`
}

type Home struct {
	GeneratedAt string      `json:"generatedAt"`
	Viewer      Viewer      `json:"viewer"`
	Search      SearchHints `json:"search"`
	Sections    []Section   `json:"sections"`
	Rails       HomeRails   `json:"rails"`
}

type HomeRails struct {
	Trending    Rail `json:"trending"`
	Dropped     Rail `json:"dropped"`
	Collections Rail `json:"collections"`
}

type ToggleResult struct {
	ID     string      `json:"id"`
	Viewer ViewerState `json:"viewer"`
	Stats  ItemStats   `json:"stats"`
	Metric Metric      `json:"metric"`
}
